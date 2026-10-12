package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/irvanmalik48/realm-api/internal/model"
	"github.com/irvanmalik48/realm-api/internal/repository"
)

var (
	ErrEmptyPasteContent = errors.New("paste content cannot be empty")
	ErrPasteTooLarge     = errors.New("paste content exceeds maximum allowed size (10MB)")
)

const maxPasteSizeBytes = 10 * 1024 * 1024 // 10MB limit for text snippet

type PasteService interface {
	CreatePaste(ctx context.Context, req *model.CreatePasteRequest, userID *uuid.UUID) (*model.Paste, error)
	GetPasteBySlug(ctx context.Context, slug string) (*model.Paste, error)
	GetRawContent(ctx context.Context, slug string) (string, error)
	ListPastes(ctx context.Context, search, syntax string, isPublicOnly *bool, userID *uuid.UUID, limit, offset int) (*model.PastesListResponse, error)
	DeletePaste(ctx context.Context, slug string) error
}

type pasteService struct {
	repo repository.PasteRepository
}

func NewPasteService(repo repository.PasteRepository) PasteService {
	return &pasteService{repo: repo}
}

func parseTTL(ttl string) *time.Time {
	ttl = strings.TrimSpace(strings.ToLower(ttl))
	if ttl == "" || ttl == "never" || ttl == "0" {
		return nil
	}

	var d time.Duration
	switch ttl {
	case "10m":
		d = 10 * time.Minute
	case "1h":
		d = 1 * time.Hour
	case "1d", "24h":
		d = 24 * time.Hour
	case "7d", "1w":
		d = 7 * 24 * time.Hour
	case "30d", "1m":
		d = 30 * 24 * time.Hour
	case "365d", "1y":
		d = 365 * 24 * time.Hour
	default:
		parsed, err := time.ParseDuration(ttl)
		if err == nil && parsed > 0 {
			d = parsed
		} else {
			return nil
		}
	}

	exp := time.Now().Add(d)
	return &exp
}

func (s *pasteService) CreatePaste(ctx context.Context, req *model.CreatePasteRequest, userID *uuid.UUID) (*model.Paste, error) {
	content := req.Content
	if strings.TrimSpace(content) == "" {
		return nil, ErrEmptyPasteContent
	}
	if len(content) > maxPasteSizeBytes {
		return nil, ErrPasteTooLarge
	}

	syntax := strings.ToLower(strings.TrimSpace(req.Syntax))
	if syntax == "" {
		syntax = "plaintext"
	}

	title := strings.TrimSpace(req.Title)

	isPublic := true
	if req.IsPublic != nil {
		isPublic = *req.IsPublic
	}

	var expiresAt *time.Time
	if req.ExpiresAt != nil && !req.ExpiresAt.IsZero() {
		expiresAt = req.ExpiresAt
	} else if req.TTL != "" {
		expiresAt = parseTTL(req.TTL)
	}

	paste := &model.Paste{
		Title:     title,
		Content:   content,
		Syntax:    syntax,
		UserID:    userID,
		IsPublic:  isPublic,
		ExpiresAt: expiresAt,
	}

	return s.repo.CreatePaste(ctx, paste)
}

func (s *pasteService) GetPasteBySlug(ctx context.Context, slug string) (*model.Paste, error) {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return nil, repository.ErrPasteNotFound
	}

	p, err := s.repo.GetPasteBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}

	// Increment view count asynchronously
	go func() {
		_ = s.repo.IncrementViews(context.Background(), slug)
	}()

	return p, nil
}

func (s *pasteService) GetRawContent(ctx context.Context, slug string) (string, error) {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return "", repository.ErrPasteNotFound
	}

	p, err := s.repo.GetPasteBySlug(ctx, slug)
	if err != nil {
		return "", err
	}

	// Increment view count asynchronously
	go func() {
		_ = s.repo.IncrementViews(context.Background(), slug)
	}()

	return p.Content, nil
}

func (s *pasteService) ListPastes(ctx context.Context, search, syntax string, isPublicOnly *bool, userID *uuid.UUID, limit, offset int) (*model.PastesListResponse, error) {
	return s.repo.ListPastes(ctx, search, syntax, isPublicOnly, userID, limit, offset)
}

func (s *pasteService) DeletePaste(ctx context.Context, slug string) error {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return repository.ErrPasteNotFound
	}
	return s.repo.DeletePaste(ctx, slug)
}
