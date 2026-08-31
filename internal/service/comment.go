package service

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/irvanmalik48/realm-api/internal/model"
	"github.com/irvanmalik48/realm-api/internal/repository"
)

const (
	MaxCommentLength = 2000
)

var (
	ErrEmptyComment   = errors.New("comment content cannot be empty")
	ErrCommentTooLong = errors.New("comment content exceeds maximum length of 2000 characters")
	ErrCommentNotFound = errors.New("comment not found")
	ErrUnauthorizedCommentAction = errors.New("unauthorized comment action")
)

type CommentService interface {
	GetComments(ctx context.Context, slug string, currentUserID *uuid.UUID) (*model.CommentsListResponse, error)
	CreateComment(ctx context.Context, slug string, userID uuid.UUID, content string, parentID *uuid.UUID) (*model.CommentDTO, error)
	UpdateComment(ctx context.Context, commentID, userID uuid.UUID, content string) (*model.CommentDTO, error)
	DeleteComment(ctx context.Context, commentID, userID uuid.UUID) error
}

type commentService struct {
	repo repository.CommentRepository
}

func NewCommentService(repo repository.CommentRepository) CommentService {
	return &commentService{repo: repo}
}

func (s *commentService) GetComments(ctx context.Context, slug string, currentUserID *uuid.UUID) (*model.CommentsListResponse, error) {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return nil, ErrEmptySlug
	}

	if s.repo == nil {
		return &model.CommentsListResponse{
			Slug:       slug,
			TotalCount: 0,
			Comments:   make([]model.CommentDTO, 0),
		}, nil
	}

	return s.repo.GetCommentsBySlug(ctx, slug, currentUserID)
}

func (s *commentService) CreateComment(ctx context.Context, slug string, userID uuid.UUID, content string, parentID *uuid.UUID) (*model.CommentDTO, error) {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return nil, ErrEmptySlug
	}

	content = strings.TrimSpace(content)
	if content == "" {
		return nil, ErrEmptyComment
	}

	if utf8.RuneCountInString(content) > MaxCommentLength {
		return nil, ErrCommentTooLong
	}

	if s.repo == nil {
		return nil, ErrDatabaseUnavailable
	}

	// If parentID provided, ensure parent comment exists and matches post slug
	if parentID != nil && *parentID != uuid.Nil {
		parent, err := s.repo.GetCommentByID(ctx, *parentID)
		if err != nil {
			return nil, errors.New("parent comment not found")
		}
		if parent.PostSlug != slug {
			return nil, errors.New("parent comment does not belong to this post")
		}
	}

	comment := &model.PostComment{
		PostSlug: slug,
		UserID:   userID,
		ParentID: parentID,
		Content:  content,
	}

	return s.repo.CreateComment(ctx, comment)
}

func (s *commentService) UpdateComment(ctx context.Context, commentID, userID uuid.UUID, content string) (*model.CommentDTO, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, ErrEmptyComment
	}

	if utf8.RuneCountInString(content) > MaxCommentLength {
		return nil, ErrCommentTooLong
	}

	if s.repo == nil {
		return nil, ErrDatabaseUnavailable
	}

	return s.repo.UpdateComment(ctx, commentID, userID, content)
}

func (s *commentService) DeleteComment(ctx context.Context, commentID, userID uuid.UUID) error {
	if s.repo == nil {
		return nil
	}

	return s.repo.DeleteComment(ctx, commentID, userID)
}
