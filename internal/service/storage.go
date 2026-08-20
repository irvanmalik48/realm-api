package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/irvanmalik48/realm-api/internal/config"
	"github.com/irvanmalik48/realm-api/internal/model"
	"github.com/irvanmalik48/realm-api/internal/repository"
	"github.com/irvanmalik48/realm-api/internal/storage"
)

var (
	ErrFileTooLarge       = errors.New("file size exceeds maximum upload limit")
	ErrUnsupportedFormat  = errors.New("unsupported image format for webp conversion")
	ErrRepositoryRequired = errors.New("database repository required for metadata tracking")
)

type StorageService interface {
	Upload(ctx context.Context, filename string, reader io.Reader, explicitContentType string) (*model.FileDTO, error)
	Get(ctx context.Context, id uuid.UUID) (*model.FileRecord, io.ReadCloser, error)
	GetAsWebP(ctx context.Context, id uuid.UUID) (*model.FileRecord, io.Reader, error)
	GetInfo(ctx context.Context, id uuid.UUID) (*model.FileDTO, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type storageService struct {
	cfg    *config.Config
	repo   repository.StorageRepository
	engine storage.Engine
}

func NewStorageService(cfg *config.Config, repo repository.StorageRepository, engine storage.Engine) StorageService {
	return &storageService{
		cfg:    cfg,
		repo:   repo,
		engine: engine,
	}
}

func (s *storageService) Upload(ctx context.Context, filename string, reader io.Reader, explicitContentType string) (*model.FileDTO, error) {
	fileID := uuid.New()

	// Read initial chunk (up to 512 bytes) for accurate MIME type detection
	sniffBuffer := make([]byte, 512)
	n, err := io.ReadFull(reader, sniffBuffer)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return nil, fmt.Errorf("failed to read file header: %w", err)
	}

	contentType := http.DetectContentType(sniffBuffer[:n])
	if explicitContentType != "" && explicitContentType != "application/octet-stream" {
		contentType = explicitContentType
	}

	// Reconstruct the full reader with size limit check
	maxBytes := int64(s.cfg.MaxUploadSizeMB) * 1024 * 1024
	var limitedReader io.Reader = io.MultiReader(bytes.NewReader(sniffBuffer[:n]), reader)
	if maxBytes > 0 {
		limitedReader = io.LimitReader(limitedReader, maxBytes+1)
	}

	// Save compressed file to disk with Zstandard
	originalSize, compressedSize, sha256Hex, err := s.engine.Save(limitedReader, fileID)
	if err != nil {
		return nil, fmt.Errorf("failed to store compressed file: %w", err)
	}

	if maxBytes > 0 && originalSize > maxBytes {
		_ = s.engine.Delete(fileID)
		return nil, fmt.Errorf("%w: %d bytes (limit: %d MB)", ErrFileTooLarge, originalSize, s.cfg.MaxUploadSizeMB)
	}

	// If image, decode to extract dimensions and calculate Blurhash
	var width *int
	var height *int
	var blurhashStr *string

	if storage.IsImageMime(contentType) {
		// Read back the decompressed file from engine to compute blurhash
		readCloser, err := s.engine.Open(fileID)
		if err == nil {
			info, imgErr := storage.ProcessImage(readCloser)
			_ = readCloser.Close()
			if imgErr == nil && info != nil {
				w := info.Width
				h := info.Height
				bh := info.Blurhash
				width = &w
				height = &h
				blurhashStr = &bh
			}
		}
	}

	sanitizedFilename := filepath.Base(filename)
	if sanitizedFilename == "" || sanitizedFilename == "." {
		sanitizedFilename = fmt.Sprintf("file-%s", fileID.String())
	}

	record := &model.FileRecord{
		ID:                   fileID,
		Filename:             sanitizedFilename,
		ContentType:          contentType,
		OriginalSize:         originalSize,
		CompressedSize:       compressedSize,
		CompressionAlgorithm: "zstd",
		SHA256:               sha256Hex,
		Blurhash:             blurhashStr,
		Width:                width,
		Height:               height,
		IsPublic:             true,
		CreatedAt:            time.Now().UTC(),
	}

	if s.repo != nil {
		if err := s.repo.Create(ctx, record); err != nil {
			_ = s.engine.Delete(fileID)
			return nil, fmt.Errorf("failed to persist file metadata: %w", err)
		}
	}

	dto := s.recordToDTO(record)
	return dto, nil
}

func (s *storageService) Get(ctx context.Context, id uuid.UUID) (*model.FileRecord, io.ReadCloser, error) {
	var record *model.FileRecord
	var err error

	if s.repo != nil {
		record, err = s.repo.GetByID(ctx, id)
		if err != nil {
			return nil, nil, err
		}
	} else {
		record = &model.FileRecord{
			ID:          id,
			Filename:    id.String(),
			ContentType: "application/octet-stream",
		}
	}

	readCloser, err := s.engine.Open(id)
	if err != nil {
		return nil, nil, err
	}

	return record, readCloser, nil
}

func (s *storageService) GetAsWebP(ctx context.Context, id uuid.UUID) (*model.FileRecord, io.Reader, error) {
	var record *model.FileRecord
	var err error

	if s.repo != nil {
		record, err = s.repo.GetByID(ctx, id)
		if err != nil {
			return nil, nil, err
		}
	} else {
		record = &model.FileRecord{
			ID:          id,
			Filename:    id.String(),
			ContentType: "application/octet-stream",
		}
	}

	if !storage.IsImageMime(record.ContentType) {
		return nil, nil, ErrUnsupportedFormat
	}

	// 1. Check if cached WebP derivative exists on disk
	cachedWebP, err := s.engine.OpenWebP(id)
	if err == nil {
		convertedRecord := *record
		convertedRecord.ContentType = "image/webp"
		convertedRecord.Filename = strings.TrimSuffix(record.Filename, filepath.Ext(record.Filename)) + ".webp"
		return &convertedRecord, cachedWebP, nil
	}

	// 2. Open original stream for conversion
	readCloser, err := s.engine.Open(id)
	if err != nil {
		return nil, nil, err
	}
	defer readCloser.Close()

	// If original is already WebP, stream directly
	if strings.Contains(strings.ToLower(record.ContentType), "webp") {
		buf := new(bytes.Buffer)
		if _, err := io.Copy(buf, readCloser); err != nil {
			return nil, nil, err
		}
		return record, buf, nil
	}

	// 3. Convert to WebP
	var webpBuf bytes.Buffer
	if err := storage.EncodeToWebP(readCloser, &webpBuf); err != nil {
		return nil, nil, fmt.Errorf("failed to convert image to webp: %w", err)
	}

	// 4. Cache converted WebP asynchronously / best-effort
	_ = s.engine.SaveWebP(id, bytes.NewReader(webpBuf.Bytes()))

	// Update record metadata for response headers
	convertedRecord := *record
	convertedRecord.ContentType = "image/webp"
	convertedRecord.Filename = strings.TrimSuffix(record.Filename, filepath.Ext(record.Filename)) + ".webp"

	return &convertedRecord, &webpBuf, nil
}

func (s *storageService) GetInfo(ctx context.Context, id uuid.UUID) (*model.FileDTO, error) {
	if s.repo == nil {
		return nil, ErrRepositoryRequired
	}

	record, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	return s.recordToDTO(record), nil
}

func (s *storageService) Delete(ctx context.Context, id uuid.UUID) error {
	if s.repo != nil {
		if err := s.repo.Delete(ctx, id); err != nil {
			return err
		}
	}

	return s.engine.Delete(id)
}

func (s *storageService) recordToDTO(record *model.FileRecord) *model.FileDTO {
	savings := 0.0
	if record.OriginalSize > 0 {
		savings = float64(record.OriginalSize-record.CompressedSize) / float64(record.OriginalSize) * 100
		if savings < 0 {
			savings = 0
		}
	}

	fileURL := fmt.Sprintf("/v1/storage/%s", record.ID.String())
	var webpURL *string
	if storage.IsImageMime(record.ContentType) {
		wURL := fmt.Sprintf("/v1/storage/%s?format=webp", record.ID.String())
		webpURL = &wURL
	}

	return &model.FileDTO{
		ID:             record.ID,
		Filename:       record.Filename,
		ContentType:    record.ContentType,
		OriginalSize:   record.OriginalSize,
		CompressedSize: record.CompressedSize,
		SavingsPercent: savings,
		SHA256:         record.SHA256,
		Blurhash:       record.Blurhash,
		Width:          record.Width,
		Height:         record.Height,
		URL:            fileURL,
		WebPURL:        webpURL,
		CreatedAt:      record.CreatedAt,
	}
}
