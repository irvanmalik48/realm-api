package model

import (
	"time"

	"github.com/google/uuid"
)

type FileRecord struct {
	ID                   uuid.UUID  `json:"id"`
	Filename             string     `json:"filename"`
	ContentType          string     `json:"content_type"`
	OriginalSize         int64      `json:"original_size"`
	CompressedSize       int64      `json:"compressed_size"`
	CompressionAlgorithm string     `json:"compression_algorithm"`
	SHA256               string     `json:"sha256"`
	Blurhash             *string    `json:"blurhash,omitempty"`
	Width                *int       `json:"width,omitempty"`
	Height               *int       `json:"height,omitempty"`
	IsPublic             bool       `json:"is_public"`
	StorageBackend       string     `json:"storage_backend"`
	S3Bucket             *string    `json:"s3_bucket,omitempty"`
	S3Key                *string    `json:"s3_key,omitempty"`
	S3ETag               *string    `json:"s3_etag,omitempty"`
	CreatedAt            time.Time  `json:"created_at"`
}

type FileDTO struct {
	ID             uuid.UUID `json:"id"`
	Filename       string    `json:"filename"`
	ContentType    string    `json:"content_type"`
	OriginalSize   int64     `json:"original_size"`
	CompressedSize int64     `json:"compressed_size"`
	SavingsPercent float64   `json:"savings_percent"`
	SHA256         string    `json:"sha256"`
	Blurhash       *string   `json:"blurhash,omitempty"`
	Width          *int      `json:"width,omitempty"`
	Height         *int      `json:"height,omitempty"`
	URL            string    `json:"url"`
	WebPURL        *string   `json:"webp_url,omitempty"`
	StorageBackend string    `json:"storage_backend"`
	S3Bucket       *string   `json:"s3_bucket,omitempty"`
	S3Key          *string   `json:"s3_key,omitempty"`
	S3ETag         *string   `json:"s3_etag,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

type FileUploadResponse struct {
	Status  string  `json:"status"`
	Message string  `json:"message"`
	File    FileDTO `json:"file"`
}

type FileInfoResponse struct {
	Status string  `json:"status"`
	File   FileDTO `json:"file"`
}

type StorageStatsDTO struct {
	ActiveBackend         string  `json:"active_backend"`
	TotalFiles            int64   `json:"total_files"`
	TotalOriginalBytes    int64   `json:"total_original_bytes"`
	TotalCompressedBytes  int64   `json:"total_compressed_bytes"`
	AverageSavingsPercent float64 `json:"average_savings_percent"`
	S3BucketName          *string `json:"s3_bucket_name,omitempty"`
}
