package service

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/irvanmalik48/realm-api/internal/model"
	"github.com/irvanmalik48/realm-api/internal/repository"
)

var (
	ErrInvalidReaction = errors.New("invalid reaction type")
	ErrEmptySlug       = errors.New("post slug cannot be empty")
)

type ReactionService interface {
	GetReactions(ctx context.Context, slug string, userID *uuid.UUID) (*model.PostReactionsResponse, error)
	ToggleReaction(ctx context.Context, slug string, reactionType string, userID uuid.UUID) (*model.ToggleReactionResponse, error)
}

type reactionService struct {
	repo repository.ReactionRepository
}

func NewReactionService(repo repository.ReactionRepository) ReactionService {
	return &reactionService{repo: repo}
}

func (s *reactionService) GetReactions(ctx context.Context, slug string, userID *uuid.UUID) (*model.PostReactionsResponse, error) {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return nil, ErrEmptySlug
	}

	if s.repo == nil {
		// Return empty response if database repository not available
		return &model.PostReactionsResponse{
			Slug:          slug,
			TotalCount:    0,
			Reactions:     make(map[string]int),
			UserReaction:  nil,
			UserReactions: make([]string, 0),
		}, nil
	}

	return s.repo.GetReactionsBySlug(ctx, slug, userID)
}

func (s *reactionService) ToggleReaction(ctx context.Context, slug string, reactionType string, userID uuid.UUID) (*model.ToggleReactionResponse, error) {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return nil, ErrEmptySlug
	}

	reactionType = strings.ToLower(strings.TrimSpace(reactionType))
	if !model.AllowedReactionTypes[reactionType] {
		return nil, ErrInvalidReaction
	}

	if s.repo == nil {
		return nil, ErrDatabaseUnavailable
	}

	return s.repo.ToggleReaction(ctx, slug, reactionType, userID)
}
