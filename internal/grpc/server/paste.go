package server

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/irvanmalik48/realm-api/internal/grpc/interceptors"
	"github.com/irvanmalik48/realm-api/internal/model"
	"github.com/irvanmalik48/realm-api/internal/repository"
	"github.com/irvanmalik48/realm-api/internal/service"
	realmv1 "github.com/irvanmalik48/realm-api/pkg/pb/realm/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type PasteServer struct {
	realmv1.UnimplementedPasteServiceServer
	pasteSvc  service.PasteService
	adminRepo repository.AdminRepository
}

func NewPasteServer(pasteSvc service.PasteService, adminRepo repository.AdminRepository) *PasteServer {
	return &PasteServer{
		pasteSvc:  pasteSvc,
		adminRepo: adminRepo,
	}
}

func mapPasteToProto(p *model.Paste) *realmv1.Paste {
	if p == nil {
		return nil
	}
	var expiresAtStr *string
	if p.ExpiresAt != nil {
		s := p.ExpiresAt.Format(time.RFC3339)
		expiresAtStr = &s
	}
	var userIDStr *string
	if p.UserID != nil {
		s := p.UserID.String()
		userIDStr = &s
	}

	return &realmv1.Paste{
		Id:          p.ID.String(),
		Slug:        p.Slug,
		Title:       p.Title,
		Content:     p.Content,
		Syntax:      p.Syntax,
		Views:       int32(p.Views),
		IsPublic:    p.IsPublic,
		IsEncrypted: p.IsEncrypted,
		UserId:      userIDStr,
		ExpiresAt:   expiresAtStr,
		CreatedAt:   p.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   p.UpdatedAt.Format(time.RFC3339),
	}
}

func mapPasteSummaryToProto(p *model.PasteSummaryDTO) *realmv1.PasteSummary {
	if p == nil {
		return nil
	}
	var expiresAtStr *string
	if p.ExpiresAt != nil {
		s := p.ExpiresAt.Format(time.RFC3339)
		expiresAtStr = &s
	}
	var userIDStr *string
	if p.UserID != nil {
		s := p.UserID.String()
		userIDStr = &s
	}

	return &realmv1.PasteSummary{
		Id:          p.ID.String(),
		Slug:        p.Slug,
		Title:       p.Title,
		Syntax:      p.Syntax,
		Views:       int32(p.Views),
		Size:        int32(p.Size),
		IsPublic:    p.IsPublic,
		IsEncrypted: p.IsEncrypted,
		UserId:      userIDStr,
		ExpiresAt:   expiresAtStr,
		CreatedAt:   p.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   p.UpdatedAt.Format(time.RFC3339),
	}
}

func (s *PasteServer) CreatePaste(ctx context.Context, req *realmv1.CreatePasteRequest) (*realmv1.Paste, error) {
	if req.GetContent() == "" {
		return nil, status.Error(codes.InvalidArgument, "Content is required and cannot be empty")
	}

	var userID *uuid.UUID
	if id, ok := interceptors.GetUserID(ctx); ok {
		userID = &id
	}

	var expiresAt *time.Time
	if req.ExpiresAt != nil && *req.ExpiresAt != "" {
		t, err := time.Parse(time.RFC3339, *req.ExpiresAt)
		if err == nil {
			expiresAt = &t
		}
	}

	createReq := &model.CreatePasteRequest{
		Title:     req.GetTitle(),
		Content:   req.GetContent(),
		Syntax:    req.GetSyntax(),
		Password:  req.GetPassword(),
		IsPublic:  req.IsPublic,
		ExpiresAt: expiresAt,
		TTL:       req.GetTtl(),
	}

	created, err := s.pasteSvc.CreatePaste(ctx, createReq, userID)
	if err != nil {
		if errors.Is(err, service.ErrEmptyPasteContent) {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		if errors.Is(err, service.ErrPasteTooLarge) {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		return nil, status.Errorf(codes.Internal, "Failed to create paste: %v", err)
	}

	return mapPasteToProto(created), nil
}

func (s *PasteServer) GetPaste(ctx context.Context, req *realmv1.GetPasteRequest) (*realmv1.Paste, error) {
	slug := req.GetSlug()
	if slug == "" {
		return nil, status.Error(codes.InvalidArgument, "Slug is required")
	}

	p, err := s.pasteSvc.GetPasteBySlug(ctx, slug, req.GetPassword())
	if err != nil {
		if errors.Is(err, repository.ErrPasteNotFound) {
			return nil, status.Error(codes.NotFound, "Paste not found")
		}
		if errors.Is(err, repository.ErrPasteExpired) {
			return nil, status.Error(codes.NotFound, "Paste has expired")
		}
		if errors.Is(err, service.ErrInvalidPassword) {
			return nil, status.Error(codes.Unauthenticated, "Invalid paste password")
		}
		return nil, status.Errorf(codes.Internal, "Failed to get paste: %v", err)
	}

	return mapPasteToProto(p), nil
}

func (s *PasteServer) GetRawPaste(ctx context.Context, req *realmv1.GetRawPasteRequest) (*realmv1.GetRawPasteResponse, error) {
	slug := req.GetSlug()
	if slug == "" {
		return nil, status.Error(codes.InvalidArgument, "Slug is required")
	}

	content, err := s.pasteSvc.GetRawContent(ctx, slug, req.GetPassword())
	if err != nil {
		if errors.Is(err, repository.ErrPasteNotFound) {
			return nil, status.Error(codes.NotFound, "Paste not found")
		}
		if errors.Is(err, repository.ErrPasteExpired) {
			return nil, status.Error(codes.NotFound, "Paste has expired")
		}
		if errors.Is(err, service.ErrPasswordRequired) {
			return nil, status.Error(codes.Unauthenticated, "Password required to view this paste")
		}
		if errors.Is(err, service.ErrInvalidPassword) {
			return nil, status.Error(codes.Unauthenticated, "Invalid paste password")
		}
		return nil, status.Errorf(codes.Internal, "Failed to get raw paste: %v", err)
	}

	return &realmv1.GetRawPasteResponse{Content: content}, nil
}

func (s *PasteServer) ListPastes(ctx context.Context, req *realmv1.ListPastesRequest) (*realmv1.ListPastesResponse, error) {
	limit := int(req.GetLimit())
	offset := int(req.GetOffset())

	var userID *uuid.UUID
	if id, ok := interceptors.GetUserID(ctx); ok {
		userID = &id
	}

	res, err := s.pasteSvc.ListPastes(ctx, req.GetSearch(), req.GetSyntax(), req.IsPublicOnly, userID, limit, offset)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to list pastes: %v", err)
	}

	protoPastes := make([]*realmv1.PasteSummary, len(res.Pastes))
	for i := range res.Pastes {
		protoPastes[i] = mapPasteSummaryToProto(&res.Pastes[i])
	}

	return &realmv1.ListPastesResponse{
		Pastes: protoPastes,
		Total:  int32(res.Total),
	}, nil
}

func (s *PasteServer) DeletePaste(ctx context.Context, req *realmv1.DeletePasteRequest) (*realmv1.DeletePasteResponse, error) {
	if err := Authorize(ctx, s.adminRepo, "pastes:delete"); err != nil {
		return nil, err
	}

	slug := req.GetSlug()
	if slug == "" {
		return nil, status.Error(codes.InvalidArgument, "Slug is required")
	}

	if err := s.pasteSvc.DeletePaste(ctx, slug); err != nil {
		if errors.Is(err, repository.ErrPasteNotFound) {
			return nil, status.Error(codes.NotFound, "Paste not found")
		}
		return nil, status.Errorf(codes.Internal, "Failed to delete paste: %v", err)
	}

	return &realmv1.DeletePasteResponse{
		Status:  "success",
		Message: "Paste deleted successfully",
	}, nil
}
