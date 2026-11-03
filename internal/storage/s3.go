package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/google/uuid"
	"github.com/irvanmalik48/realm-api/internal/config"
	"github.com/klauspost/compress/zstd"
)

type S3Engine interface {
	Engine
	GeneratePresignedURL(ctx context.Context, id uuid.UUID, expiry time.Duration) (string, error)
	BucketName() string
	PublicURL(id uuid.UUID) string
	EnsureBucketExists(ctx context.Context) error
}

type s3Engine struct {
	client        *s3.Client
	presignClient *s3.PresignClient
	bucket        string
	publicDomain  string
}

func NewS3Engine(cfg *config.Config) (S3Engine, error) {
	if cfg.S3Bucket == "" {
		return nil, errors.New("S3_BUCKET is required when using s3 storage backend")
	}

	opts := []func(*awsConfig.LoadOptions) error{
		awsConfig.WithRegion(cfg.S3Region),
	}

	if cfg.S3AccessKeyID != "" && cfg.S3SecretAccessKey != "" {
		opts = append(opts, awsConfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.S3AccessKeyID, cfg.S3SecretAccessKey, ""),
		))
	}

	awsCfg, err := awsConfig.LoadDefaultConfig(context.Background(), opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to load aws config: %w", err)
	}

	s3Client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if cfg.S3Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.S3Endpoint)
		}
		o.UsePathStyle = cfg.S3UsePathStyle
	})

	presignClient := s3.NewPresignClient(s3Client)

	engine := &s3Engine{
		client:        s3Client,
		presignClient: presignClient,
		bucket:        cfg.S3Bucket,
		publicDomain:  cfg.S3PublicDomain,
	}

	// Asynchronously verify or create bucket with retries while S3 backend boots
	go func() {
		for i := 0; i < 5; i++ {
			initCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			ensureErr := engine.EnsureBucketExists(initCtx)
			cancel()
			if ensureErr == nil {
				return
			}
			time.Sleep(2 * time.Second)
		}
	}()

	return engine, nil
}

func (e *s3Engine) BucketName() string {
	return e.bucket
}

func (e *s3Engine) FilePath(id uuid.UUID) string {
	return id.String() + ".zst"
}

func (e *s3Engine) webpKey(id uuid.UUID) string {
	return id.String() + ".webp.zst"
}

func (e *s3Engine) EnsureBucketExists(ctx context.Context) error {
	_, err := e.client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(e.bucket),
	})
	if err == nil {
		return nil
	}

	createCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, createErr := e.client.CreateBucket(createCtx, &s3.CreateBucketInput{
		Bucket: aws.String(e.bucket),
	})
	if createErr != nil {
		var bne *types.BucketAlreadyExists
		var boe *types.BucketAlreadyOwnedByYou
		if errors.As(createErr, &bne) || errors.As(createErr, &boe) {
			return nil
		}
		return fmt.Errorf("failed to create bucket %q: %w", e.bucket, createErr)
	}

	return nil
}

func (e *s3Engine) Save(reader io.Reader, id uuid.UUID) (int64, int64, string, error) {
	var compressedBuf bytes.Buffer
	hash := sha256.New()
	countingReader := &countingReader{reader: reader, hash: hash}

	zw := zstdWriterPool.Get().(*zstd.Encoder)
	defer zstdWriterPool.Put(zw)
	zw.Reset(&compressedBuf)

	if _, err := io.Copy(zw, countingReader); err != nil {
		_ = zw.Close()
		return 0, 0, "", fmt.Errorf("failed to compress stream for S3: %w", err)
	}

	if err := zw.Close(); err != nil {
		return 0, 0, "", fmt.Errorf("failed to finalize zstd for S3: %w", err)
	}

	compressedData := compressedBuf.Bytes()
	key := e.FilePath(id)

	putInput := &s3.PutObjectInput{
		Bucket:          aws.String(e.bucket),
		Key:             aws.String(key),
		Body:            bytes.NewReader(compressedData),
		ContentLength:   aws.Int64(int64(len(compressedData))),
		ContentEncoding: aws.String("zstd"),
	}

	_, err := e.client.PutObject(context.Background(), putInput)
	if err != nil {
		// Attempt to auto-create bucket in case it didn't exist yet, then retry PutObject once
		if createErr := e.EnsureBucketExists(context.Background()); createErr == nil {
			putInput.Body = bytes.NewReader(compressedData)
			_, err = e.client.PutObject(context.Background(), putInput)
		}
	}
	if err != nil {
		return 0, 0, "", fmt.Errorf("failed to upload object to S3: %w", err)
	}

	sha256Hex := hex.EncodeToString(hash.Sum(nil))
	return countingReader.bytesRead, int64(len(compressedData)), sha256Hex, nil
}

func (e *s3Engine) Open(id uuid.UUID) (io.ReadCloser, error) {
	key := e.FilePath(id)
	resp, err := e.client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket: aws.String(e.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get object from S3: %w", err)
	}

	zr := zstdReaderPool.Get().(*zstd.Decoder)
	if err := zr.Reset(resp.Body); err != nil {
		zstdReaderPool.Put(zr)
		_ = resp.Body.Close()
		return nil, fmt.Errorf("failed to open zstd stream from S3: %w", err)
	}

	return &s3ZstdReadCloser{
		reader:   zr,
		s3Closer: resp.Body,
	}, nil
}

func (e *s3Engine) SaveWebP(id uuid.UUID, reader io.Reader) error {
	var compressedBuf bytes.Buffer
	zw := zstdWriterPool.Get().(*zstd.Encoder)
	defer zstdWriterPool.Put(zw)
	zw.Reset(&compressedBuf)

	if _, err := io.Copy(zw, reader); err != nil {
		_ = zw.Close()
		return fmt.Errorf("failed to compress webp for S3: %w", err)
	}

	if err := zw.Close(); err != nil {
		return fmt.Errorf("failed to finalize webp zstd for S3: %w", err)
	}

	compressedData := compressedBuf.Bytes()
	key := e.webpKey(id)

	putInput := &s3.PutObjectInput{
		Bucket:          aws.String(e.bucket),
		Key:             aws.String(key),
		Body:            bytes.NewReader(compressedData),
		ContentLength:   aws.Int64(int64(len(compressedData))),
		ContentEncoding: aws.String("zstd"),
	}

	_, err := e.client.PutObject(context.Background(), putInput)
	if err != nil {
		if createErr := e.EnsureBucketExists(context.Background()); createErr == nil {
			putInput.Body = bytes.NewReader(compressedData)
			_, err = e.client.PutObject(context.Background(), putInput)
		}
	}
	if err != nil {
		return fmt.Errorf("failed to upload webp to S3: %w", err)
	}
	return nil
}

func (e *s3Engine) OpenWebP(id uuid.UUID) (io.ReadCloser, error) {
	key := e.webpKey(id)
	resp, err := e.client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket: aws.String(e.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get webp from S3: %w", err)
	}

	zr := zstdReaderPool.Get().(*zstd.Decoder)
	if err := zr.Reset(resp.Body); err != nil {
		zstdReaderPool.Put(zr)
		_ = resp.Body.Close()
		return nil, fmt.Errorf("failed to open webp zstd stream from S3: %w", err)
	}

	return &s3ZstdReadCloser{
		reader:   zr,
		s3Closer: resp.Body,
	}, nil
}

func (e *s3Engine) Delete(id uuid.UUID) error {
	key := e.FilePath(id)
	webpKey := e.webpKey(id)

	_, _ = e.client.DeleteObject(context.Background(), &s3.DeleteObjectInput{
		Bucket: aws.String(e.bucket),
		Key:    aws.String(webpKey),
	})

	_, err := e.client.DeleteObject(context.Background(), &s3.DeleteObjectInput{
		Bucket: aws.String(e.bucket),
		Key:    aws.String(key),
	})
	return err
}

func (e *s3Engine) PublicURL(id uuid.UUID) string {
	if e.publicDomain == "" {
		return ""
	}
	base := strings.TrimRight(e.publicDomain, "/")
	return fmt.Sprintf("%s/%s/%s", base, e.bucket, e.FilePath(id))
}

func (e *s3Engine) GeneratePresignedURL(ctx context.Context, id uuid.UUID, expiry time.Duration) (string, error) {
	key := e.FilePath(id)
	if expiry <= 0 {
		expiry = 15 * time.Minute
	}

	req, err := e.presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(e.bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(expiry))
	if err != nil {
		return "", fmt.Errorf("failed to presign get object: %w", err)
	}

	resURL := req.URL
	if e.publicDomain != "" {
		if u, parseErr := url.Parse(req.URL); parseErr == nil {
			if pubU, parseErr := url.Parse(e.publicDomain); parseErr == nil {
				u.Scheme = pubU.Scheme
				u.Host = pubU.Host
				resURL = u.String()
			}
		}
	}

	return resURL, nil
}

type s3ZstdReadCloser struct {
	reader   *zstd.Decoder
	s3Closer io.ReadCloser
}

func (z *s3ZstdReadCloser) Read(p []byte) (n int, err error) {
	return z.reader.Read(p)
}

func (z *s3ZstdReadCloser) Close() error {
	err := z.s3Closer.Close()
	zstdReaderPool.Put(z.reader)
	return err
}
