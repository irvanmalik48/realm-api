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
	GetImageMetadataFromURL(ctx context.Context, targetURL string) (*storage.ImageMetadata, error)
	List(ctx context.Context, limit, offset int, search, backend string) ([]model.FileDTO, int, error)
	GetStorageStats(ctx context.Context) (*model.StorageStatsDTO, error)
	GeneratePresignedURL(ctx context.Context, id uuid.UUID, expiry time.Duration) (string, time.Time, error)
}

type storageService struct {
	cfg         *config.Config
	repo        repository.StorageRepository
	engine      storage.Engine
	localEngine storage.Engine
}

func NewStorageService(cfg *config.Config, repo repository.StorageRepository, engine storage.Engine, fallbackEngine ...storage.Engine) StorageService {
	var localEng storage.Engine
	if len(fallbackEngine) > 0 {
		localEng = fallbackEngine[0]
	} else if cfg != nil && cfg.StorageDir != "" {
		if _, isS3 := engine.(storage.S3Engine); isS3 {
			localEng, _ = storage.NewZstdEngine(cfg.StorageDir)
		}
	}

	return &storageService{
		cfg:         cfg,
		repo:        repo,
		engine:      engine,
		localEngine: localEng,
	}
}

func (s *storageService) resolveEngine(record *model.FileRecord) storage.Engine {
	if record != nil && record.StorageBackend == "local" && s.localEngine != nil {
		return s.localEngine
	}
	return s.engine
}

func (s *storageService) Upload(ctx context.Context, filename string, reader io.Reader, explicitContentType string) (*model.FileDTO, error) {
	fileID := uuid.New()

	// Read initial chunk (up to 512 bytes) for accurate MIME type detection
	sniffBuffer := make([]byte, 512)
	n, err := io.ReadFull(reader, sniffBuffer)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return nil, fmt.Errorf("failed to read file header: %w", err)
	}

	detectedType := http.DetectContentType(sniffBuffer[:n])
	contentType := detectedType
	if explicitContentType != "" && explicitContentType != "application/octet-stream" {
		if storage.IsImageMime(explicitContentType) {
			contentType = explicitContentType
		}
	}
	// Neutralize dangerous types (HTML, JavaScript, SVG with embedded scripts)
	lowerType := strings.ToLower(contentType)
	if strings.Contains(lowerType, "html") ||
		strings.Contains(lowerType, "javascript") ||
		strings.Contains(lowerType, "script") ||
		strings.Contains(lowerType, "svg") {
		contentType = "application/octet-stream"
	}

	// Reconstruct the full reader with size limit check
	maxBytes := int64(s.cfg.MaxUploadSizeMB) * 1024 * 1024
	limitedReader := io.MultiReader(bytes.NewReader(sniffBuffer[:n]), reader)
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
		sanitizedFilename = "file-" + fileID.String()
	}

	backend := "local"
	var s3Bucket, s3Key *string
	if s3Eng, ok := s.engine.(storage.S3Engine); ok {
		backend = "s3"
		b := s3Eng.BucketName()
		k := s3Eng.FilePath(fileID)
		s3Bucket = &b
		s3Key = &k
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
		StorageBackend:       backend,
		S3Bucket:             s3Bucket,
		S3Key:                s3Key,
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

	engine := s.resolveEngine(record)
	readCloser, err := engine.Open(id)
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

	engine := s.resolveEngine(record)

	// 1. Check if cached WebP derivative exists on disk
	cachedWebP, err := engine.OpenWebP(id)
	if err == nil {
		convertedRecord := *record
		convertedRecord.ContentType = "image/webp"
		convertedRecord.Filename = strings.TrimSuffix(record.Filename, filepath.Ext(record.Filename)) + ".webp"
		return &convertedRecord, cachedWebP, nil
	}

	// 2. Open original stream for conversion
	readCloser, err := engine.Open(id)
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
	_ = engine.SaveWebP(id, bytes.NewReader(webpBuf.Bytes()))

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
	targetEngine := s.engine
	if s.repo != nil {
		record, err := s.repo.GetByID(ctx, id)
		if err == nil {
			targetEngine = s.resolveEngine(record)
		}
		if err := s.repo.Delete(ctx, id); err != nil {
			return err
		}
	}

	return targetEngine.Delete(id)
}

func (s *storageService) recordToDTO(record *model.FileRecord) *model.FileDTO {
	savings := 0.0
	if record.OriginalSize > 0 {
		savings = float64(record.OriginalSize-record.CompressedSize) / float64(record.OriginalSize) * 100
		if savings < 0 {
			savings = 0
		}
	}

	fileURL := "/v1/storage/" + record.ID.String()
	var webpURL *string
	if storage.IsImageMime(record.ContentType) {
		wURL := "/v1/storage/" + record.ID.String() + "?format=webp"
		webpURL = &wURL
	}

	var s3URL *string
	if record.StorageBackend == "s3" && s.cfg != nil && s.cfg.S3PublicDomain != "" {
		domain := strings.TrimRight(s.cfg.S3PublicDomain, "/")
		key := record.ID.String() + ".zst"
		if record.S3Key != nil && *record.S3Key != "" {
			key = *record.S3Key
		}
		var full string
		if record.S3Bucket != nil && *record.S3Bucket != "" && !strings.Contains(domain, *record.S3Bucket) {
			full = fmt.Sprintf("%s/%s/%s", domain, *record.S3Bucket, key)
		} else {
			full = fmt.Sprintf("%s/%s", domain, key)
		}
		s3URL = &full
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
		StorageBackend: record.StorageBackend,
		S3Bucket:       record.S3Bucket,
		S3Key:          record.S3Key,
		S3ETag:         record.S3ETag,
		S3URL:          s3URL,
		CreatedAt:      record.CreatedAt,
	}
}

func (s *storageService) List(ctx context.Context, limit, offset int, search, backend string) ([]model.FileDTO, int, error) {
	if s.repo == nil {
		return nil, 0, ErrRepositoryRequired
	}

	records, total, err := s.repo.List(ctx, limit, offset, search, backend)
	if err != nil {
		return nil, 0, err
	}

	dtos := make([]model.FileDTO, len(records))
	for i := range records {
		dtos[i] = *s.recordToDTO(&records[i])
	}

	return dtos, total, nil
}

func (s *storageService) GetStorageStats(ctx context.Context) (*model.StorageStatsDTO, error) {
	if s.repo == nil {
		return nil, ErrRepositoryRequired
	}

	totalFiles, totalOrig, totalComp, avgSavings, err := s.repo.GetStats(ctx)
	if err != nil {
		return nil, err
	}

	activeBackend := "local"
	var bucketName *string
	if s3Eng, ok := s.engine.(storage.S3Engine); ok {
		activeBackend = "s3"
		b := s3Eng.BucketName()
		bucketName = &b
	}

	return &model.StorageStatsDTO{
		ActiveBackend:         activeBackend,
		TotalFiles:            totalFiles,
		TotalOriginalBytes:    totalOrig,
		TotalCompressedBytes:  totalComp,
		AverageSavingsPercent: avgSavings,
		S3BucketName:          bucketName,
	}, nil
}

func (s *storageService) GeneratePresignedURL(ctx context.Context, id uuid.UUID, expiry time.Duration) (string, time.Time, error) {
	s3Eng, ok := s.engine.(storage.S3Engine)
	if !ok {
		return "", time.Time{}, errors.New("presigned URLs are only supported on S3 storage backend")
	}

	url, err := s3Eng.GeneratePresignedURL(ctx, id, expiry)
	if err != nil {
		return "", time.Time{}, err
	}

	return url, time.Now().Add(expiry), nil
}

func (s *storageService) GetImageMetadataFromURL(ctx context.Context, targetURL string) (*storage.ImageMetadata, error) {
	return storage.FetchAndProcessImage(ctx, targetURL)
}
