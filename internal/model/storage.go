package model

import (
	"time"

	"github.com/google/uuid"
)

type FileRecord struct {
	CreatedAt            time.Time `json:"created_at"`
	S3Bucket             *string   `json:"s3_bucket,omitempty"`
	Blurhash             *string   `json:"blurhash,omitempty"`
	S3ETag               *string   `json:"s3_etag,omitempty"`
	S3Key                *string   `json:"s3_key,omitempty"`
	Height               *int      `json:"height,omitempty"`
	Width                *int      `json:"width,omitempty"`
	CompressionAlgorithm string    `json:"compression_algorithm"`
	StorageBackend       string    `json:"storage_backend"`
	ContentType          string    `json:"content_type"`
	SHA256               string    `json:"sha256"`
	Filename             string    `json:"filename"`
	CompressedSize       int64     `json:"compressed_size"`
	OriginalSize         int64     `json:"original_size"`
	ID                   uuid.UUID `json:"id"`
	IsPublic             bool      `json:"is_public"`
}

type FileDTO struct {
	CreatedAt      time.Time `json:"created_at"`
	Width          *int      `json:"width,omitempty"`
	S3ETag         *string   `json:"s3_etag,omitempty"`
	S3Key          *string   `json:"s3_key,omitempty"`
	S3Bucket       *string   `json:"s3_bucket,omitempty"`
	WebPURL        *string   `json:"webp_url,omitempty"`
	Height         *int      `json:"height,omitempty"`
	Blurhash       *string   `json:"blurhash,omitempty"`
	StorageBackend string    `json:"storage_backend"`
	SHA256         string    `json:"sha256"`
	URL            string    `json:"url"`
	ContentType    string    `json:"content_type"`
	Filename       string    `json:"filename"`
	SavingsPercent float64   `json:"savings_percent"`
	CompressedSize int64     `json:"compressed_size"`
	OriginalSize   int64     `json:"original_size"`
	ID             uuid.UUID `json:"id"`
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
	S3BucketName          *string `json:"s3_bucket_name,omitempty"`
	ActiveBackend         string  `json:"active_backend"`
	TotalFiles            int64   `json:"total_files"`
	TotalOriginalBytes    int64   `json:"total_original_bytes"`
	TotalCompressedBytes  int64   `json:"total_compressed_bytes"`
	AverageSavingsPercent float64 `json:"average_savings_percent"`
}
