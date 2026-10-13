package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/irvanmalik48/realm-api/internal/model"
	"github.com/irvanmalik48/realm-api/internal/repository"
	"github.com/irvanmalik48/realm-api/internal/security"
)

var (
	ErrEmptyPasteContent = errors.New("paste content cannot be empty")
	ErrPasteTooLarge     = errors.New("paste content exceeds maximum allowed size (10MB)")
	ErrPasswordRequired  = security.ErrPasswordRequired
	ErrInvalidPassword   = security.ErrInvalidPassword
)

const maxPasteSizeBytes = 10 * 1024 * 1024 // 10MB limit for text snippet

type PasteService interface {
	CreatePaste(ctx context.Context, req *model.CreatePasteRequest, userID *uuid.UUID) (*model.Paste, error)
	GetPasteBySlug(ctx context.Context, slug string, password string) (*model.Paste, error)
	GetRawContent(ctx context.Context, slug string, password string) (string, error)
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

	isEncrypted := false
	storedContent := content
	if password := strings.TrimSpace(req.Password); password != "" {
		encryptedPayload, err := security.EncryptPaste(content, password)
		if err != nil {
			return nil, err
		}
		storedContent = encryptedPayload
		isEncrypted = true
	}

	paste := &model.Paste{
		Title:       title,
		Content:     storedContent,
		Syntax:      syntax,
		UserID:      userID,
		IsPublic:    isPublic,
		IsEncrypted: isEncrypted,
		ExpiresAt:   expiresAt,
	}

	created, err := s.repo.CreatePaste(ctx, paste)
	if err != nil {
		return nil, err
	}

	// For the creator upon creation, return the original content
	if isEncrypted {
		created.Content = content
	}

	return created, nil
}

func (s *pasteService) GetPasteBySlug(ctx context.Context, slug string, password string) (*model.Paste, error) {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return nil, repository.ErrPasteNotFound
	}

	p, err := s.repo.GetPasteBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}

	if p.IsEncrypted {
		if password == "" {
			// Password not supplied: return empty content to signal lock without exposing raw ciphertext
			lockedPaste := *p
			lockedPaste.Content = ""
			return &lockedPaste, nil
		}

		decrypted, err := security.DecryptPaste(p.Content, password)
		if err != nil {
			return nil, err
		}
		p.Content = decrypted
	}

	// Increment view count asynchronously
	go func() {
		_ = s.repo.IncrementViews(context.Background(), slug)
	}()

	return p, nil
}

func (s *pasteService) GetRawContent(ctx context.Context, slug string, password string) (string, error) {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return "", repository.ErrPasteNotFound
	}

	p, err := s.repo.GetPasteBySlug(ctx, slug)
	if err != nil {
		return "", err
	}

	content := p.Content
	if p.IsEncrypted {
		if password == "" {
			return "", ErrPasswordRequired
		}
		decrypted, err := security.DecryptPaste(p.Content, password)
		if err != nil {
			return "", err
		}
		content = decrypted
	}

	// Increment view count asynchronously
	go func() {
		_ = s.repo.IncrementViews(context.Background(), slug)
	}()

	return content, nil
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
