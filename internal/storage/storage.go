package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/klauspost/compress/zstd"
)

var (
	ErrFileNotFound = errors.New("storage file not found")
)

type Engine interface {
	Save(reader io.Reader, id uuid.UUID) (originalSize int64, compressedSize int64, sha256Hex string, err error)
	Open(id uuid.UUID) (io.ReadCloser, error)
	SaveWebP(id uuid.UUID, reader io.Reader) error
	OpenWebP(id uuid.UUID) (io.ReadCloser, error)
	Delete(id uuid.UUID) error
	FilePath(id uuid.UUID) string
}

type zstdEngine struct {
	storageDir string
}

func NewZstdEngine(storageDir string) (Engine, error) {
	if storageDir == "" {
		storageDir = "./data/storage"
	}

	if err := os.MkdirAll(storageDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}

	return &zstdEngine{
		storageDir: storageDir,
	}, nil
}

func (e *zstdEngine) FilePath(id uuid.UUID) string {
	return filepath.Join(e.storageDir, fmt.Sprintf("%s.zst", id.String()))
}

func (e *zstdEngine) webpFilePath(id uuid.UUID) string {
	return filepath.Join(e.storageDir, fmt.Sprintf("%s.webp.zst", id.String()))
}

func (e *zstdEngine) Save(reader io.Reader, id uuid.UUID) (int64, int64, string, error) {
	targetPath := e.FilePath(id)

	outFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return 0, 0, "", fmt.Errorf("failed to create storage file: %w", err)
	}
	defer outFile.Close()

	hash := sha256.New()
	countingReader := &countingReader{reader: reader, hash: hash}

	zstdWriter, err := zstd.NewWriter(outFile, zstd.WithEncoderLevel(zstd.SpeedDefault))
	if err != nil {
		return 0, 0, "", fmt.Errorf("failed to create zstd writer: %w", err)
	}

	if _, err := io.Copy(zstdWriter, countingReader); err != nil {
		_ = zstdWriter.Close()
		_ = os.Remove(targetPath)
		return 0, 0, "", fmt.Errorf("failed to compress and write file: %w", err)
	}

	if err := zstdWriter.Close(); err != nil {
		_ = os.Remove(targetPath)
		return 0, 0, "", fmt.Errorf("failed to finalize zstd compression: %w", err)
	}

	stat, err := outFile.Stat()
	if err != nil {
		return 0, 0, "", fmt.Errorf("failed to stat compressed file: %w", err)
	}

	sha256Hex := hex.EncodeToString(hash.Sum(nil))
	return countingReader.bytesRead, stat.Size(), sha256Hex, nil
}

func (e *zstdEngine) Open(id uuid.UUID) (io.ReadCloser, error) {
	targetPath := e.FilePath(id)

	file, err := os.Open(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrFileNotFound
		}
		return nil, err
	}

	zstdReader, err := zstd.NewReader(file)
	if err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("failed to open zstd stream: %w", err)
	}

	return &zstdReadCloser{
		reader: zstdReader,
		file:   file,
	}, nil
}

func (e *zstdEngine) SaveWebP(id uuid.UUID, reader io.Reader) error {
	targetPath := e.webpFilePath(id)

	outFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to create webp storage file: %w", err)
	}
	defer outFile.Close()

	zstdWriter, err := zstd.NewWriter(outFile, zstd.WithEncoderLevel(zstd.SpeedDefault))
	if err != nil {
		return fmt.Errorf("failed to create zstd writer for webp: %w", err)
	}

	if _, err := io.Copy(zstdWriter, reader); err != nil {
		_ = zstdWriter.Close()
		_ = os.Remove(targetPath)
		return fmt.Errorf("failed to compress and write webp file: %w", err)
	}

	if err := zstdWriter.Close(); err != nil {
		_ = os.Remove(targetPath)
		return fmt.Errorf("failed to finalize webp zstd compression: %w", err)
	}

	return nil
}

func (e *zstdEngine) OpenWebP(id uuid.UUID) (io.ReadCloser, error) {
	targetPath := e.webpFilePath(id)

	file, err := os.Open(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrFileNotFound
		}
		return nil, err
	}

	zstdReader, err := zstd.NewReader(file)
	if err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("failed to open webp zstd stream: %w", err)
	}

	return &zstdReadCloser{
		reader: zstdReader,
		file:   file,
	}, nil
}

func (e *zstdEngine) Delete(id uuid.UUID) error {
	targetPath := e.FilePath(id)
	webpPath := e.webpFilePath(id)
	_ = os.Remove(webpPath)

	if err := os.Remove(targetPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return nil
}

type countingReader struct {
	reader    io.Reader
	hash      io.Writer
	bytesRead int64
}

func (c *countingReader) Read(p []byte) (n int, err error) {
	n, err = c.reader.Read(p)
	if n > 0 {
		c.bytesRead += int64(n)
		_, _ = c.hash.Write(p[:n])
	}
	return n, err
}

type zstdReadCloser struct {
	reader *zstd.Decoder
	file   *os.File
}

func (z *zstdReadCloser) Read(p []byte) (n int, err error) {
	return z.reader.Read(p)
}

func (z *zstdReadCloser) Close() error {
	z.reader.Close()
	return z.file.Close()
}
