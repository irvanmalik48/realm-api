package server

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"
	"github.com/irvanmalik48/realm-api/internal/config"
	"github.com/irvanmalik48/realm-api/internal/model"
	"github.com/irvanmalik48/realm-api/internal/service"
	realmv1 "github.com/irvanmalik48/realm-api/pkg/pb/realm/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type StorageServer struct {
	realmv1.UnimplementedStorageServiceServer
	cfg        *config.Config
	storageSvc service.StorageService
}

func NewStorageServer(cfg *config.Config, storageSvc service.StorageService) *StorageServer {
	return &StorageServer{
		cfg:        cfg,
		storageSvc: storageSvc,
	}
}

func mapFileDTOToProto(dto *model.FileDTO) *realmv1.FileMetadata {
	if dto == nil {
		return nil
	}

	var w, h *int32
	if dto.Width != nil {
		widthVal := int32(*dto.Width)
		w = &widthVal
	}
	if dto.Height != nil {
		heightVal := int32(*dto.Height)
		h = &heightVal
	}

	return &realmv1.FileMetadata{
		Id:             dto.ID.String(),
		Filename:       dto.Filename,
		ContentType:    dto.ContentType,
		OriginalSize:   dto.OriginalSize,
		CompressedSize: dto.CompressedSize,
		SavingsPercent: dto.SavingsPercent,
		Sha256:         dto.SHA256,
		Blurhash:       dto.Blurhash,
		Width:          w,
		Height:         h,
		Url:            dto.URL,
		WebpUrl:        dto.WebPURL,
		CreatedAt:      dto.CreatedAt.Format(time.RFC3339),
	}
}

func (s *StorageServer) UploadFile(ctx context.Context, req *realmv1.UploadFileRequest) (*realmv1.UploadFileResponse, error) {
	if len(req.GetData()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "File data cannot be empty")
	}

	filename := req.GetFilename()
	if filename == "" {
		filename = "upload.bin"
	}

	reader := bytes.NewReader(req.GetData())
	fileDTO, err := s.storageSvc.Upload(ctx, filename, reader, req.GetContentType())
	if err != nil {
		return nil, err
	}

	return &realmv1.UploadFileResponse{
		Status:  "success",
		Message: "File uploaded and compressed successfully",
		File:    mapFileDTOToProto(fileDTO),
	}, nil
}

func (s *StorageServer) GetFileInfo(ctx context.Context, req *realmv1.GetFileInfoRequest) (*realmv1.GetFileInfoResponse, error) {
	fileID, err := uuid.Parse(req.GetId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "Invalid file UUID")
	}

	dto, err := s.storageSvc.GetInfo(ctx, fileID)
	if err != nil {
		return nil, err
	}

	return &realmv1.GetFileInfoResponse{
		Status: "success",
		File:   mapFileDTOToProto(dto),
	}, nil
}

func (s *StorageServer) DeleteFile(ctx context.Context, req *realmv1.DeleteFileRequest) (*realmv1.DeleteFileResponse, error) {
	fileID, err := uuid.Parse(req.GetId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "Invalid file UUID")
	}

	if err := s.storageSvc.Delete(ctx, fileID); err != nil {
		return nil, err
	}

	return &realmv1.DeleteFileResponse{
		Status:  "success",
		Message: "File deleted successfully",
	}, nil
}

func (s *StorageServer) GetFile(req *realmv1.GetFileRequest, stream realmv1.StorageService_GetFileServer) error {
	fileID, err := uuid.Parse(req.GetId())
	if err != nil {
		return status.Error(codes.InvalidArgument, "Invalid file UUID")
	}

	ctx := stream.Context()
	var record *model.FileRecord
	var rdr io.Reader

	if req.GetFormat() == "webp" {
		rec, streamReader, err := s.storageSvc.GetAsWebP(ctx, fileID)
		if err == nil {
			record = rec
			rdr = streamReader
		}
	}

	if record == nil {
		rec, readCloser, err := s.storageSvc.Get(ctx, fileID)
		if err != nil {
			return err
		}
		defer readCloser.Close()
		record = rec
		rdr = readCloser
	}

	buf := make([]byte, 64*1024) // 64KB chunk
	for {
		n, err := rdr.Read(buf)
		if n > 0 {
			if sendErr := stream.Send(&realmv1.FileChunkResponse{
				Chunk:       buf[:n],
				ContentType: record.ContentType,
				Filename:    record.Filename,
			}); sendErr != nil {
				return fmt.Errorf("failed to stream chunk: %w", sendErr)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read file data: %w", err)
		}
	}

	return nil
}
