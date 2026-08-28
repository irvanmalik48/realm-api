package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/irvanmalik48/realm-api/internal/database"
	"github.com/irvanmalik48/realm-api/internal/model"
)

type CommentRepository interface {
	GetCommentsBySlug(ctx context.Context, slug string, currentUserID *uuid.UUID) (*model.CommentsListResponse, error)
	CreateComment(ctx context.Context, comment *model.PostComment) (*model.CommentDTO, error)
	UpdateComment(ctx context.Context, commentID, userID uuid.UUID, content string) (*model.CommentDTO, error)
	DeleteComment(ctx context.Context, commentID, userID uuid.UUID) error
	GetCommentByID(ctx context.Context, commentID uuid.UUID) (*model.PostComment, error)
}

type commentRepository struct {
	db *database.DB
}

func NewCommentRepository(db *database.DB) CommentRepository {
	return &commentRepository{db: db}
}

func (r *commentRepository) GetCommentsBySlug(ctx context.Context, slug string, currentUserID *uuid.UUID) (*model.CommentsListResponse, error) {
	query := `
		SELECT 
			c.id, c.post_slug, c.user_id, c.parent_id, c.content, c.is_edited, c.created_at, c.updated_at,
			u.id, u.username, u.full_name, u.avatar_url
		FROM post_comments c
		JOIN users u ON c.user_id = u.id
		WHERE c.post_slug = $1
		ORDER BY c.created_at ASC
	`
	rows, err := r.db.Pool.Query(ctx, query, slug)
	if err != nil {
		return nil, fmt.Errorf("failed to query comments: %w", err)
	}
	defer rows.Close()

	var allComments []model.CommentDTO
	totalCount := 0

	for rows.Next() {
		var c model.CommentDTO
		var uID uuid.UUID
		var uName, uFullName string
		var uAvatarURL *string
		var cUserID uuid.UUID

		if err := rows.Scan(
			&c.ID, &c.PostSlug, &cUserID, &c.ParentID, &c.Content, &c.IsEdited, &c.CreatedAt, &c.UpdatedAt,
			&uID, &uName, &uFullName, &uAvatarURL,
		); err != nil {
			return nil, fmt.Errorf("failed to scan comment row: %w", err)
		}

		c.Author = model.CommentAuthorDTO{
			ID:        uID,
			Username:  uName,
			FullName:  uFullName,
			AvatarURL: uAvatarURL,
		}
		c.Replies = make([]model.CommentDTO, 0)
		if currentUserID != nil && *currentUserID != uuid.Nil {
			c.IsAuthor = (*currentUserID == cUserID)
		}

		allComments = append(allComments, c)
		totalCount++
	}

	// Group into nested structure (top-level with replies)
	commentMap := make(map[uuid.UUID]*model.CommentDTO)
	for i := range allComments {
		commentMap[allComments[i].ID] = &allComments[i]
	}

	var rootComments []model.CommentDTO
	for i := range allComments {
		item := allComments[i]
		if item.ParentID != nil {
			if parent, exists := commentMap[*item.ParentID]; exists {
				parent.Replies = append(parent.Replies, item)
			} else {
				rootComments = append(rootComments, item)
			}
		} else {
			rootComments = append(rootComments, item)
		}
	}

	// Update root comment replies from pointers
	for i := range rootComments {
		if ptr, ok := commentMap[rootComments[i].ID]; ok {
			rootComments[i].Replies = ptr.Replies
		}
	}

	if rootComments == nil {
		rootComments = make([]model.CommentDTO, 0)
	}

	return &model.CommentsListResponse{
		Slug:       slug,
		TotalCount: totalCount,
		Comments:   rootComments,
	}, nil
}

func (r *commentRepository) CreateComment(ctx context.Context, comment *model.PostComment) (*model.CommentDTO, error) {
	query := `
		INSERT INTO post_comments (post_slug, user_id, parent_id, content)
		VALUES ($1, $2, $3, $4)
		RETURNING id, is_edited, created_at, updated_at
	`
	err := r.db.Pool.QueryRow(ctx, query, comment.PostSlug, comment.UserID, comment.ParentID, comment.Content).
		Scan(&comment.ID, &comment.IsEdited, &comment.CreatedAt, &comment.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to insert comment: %w", err)
	}

	// Fetch author details
	var uName, uFullName string
	var uAvatarURL *string
	userQuery := `SELECT username, full_name, avatar_url FROM users WHERE id = $1`
	if err := r.db.Pool.QueryRow(ctx, userQuery, comment.UserID).Scan(&uName, &uFullName, &uAvatarURL); err != nil {
		return nil, fmt.Errorf("failed to query author info: %w", err)
	}

	return &model.CommentDTO{
		ID:        comment.ID,
		PostSlug:  comment.PostSlug,
		ParentID:  comment.ParentID,
		Content:   comment.Content,
		IsEdited:  comment.IsEdited,
		IsAuthor:  true,
		Author: model.CommentAuthorDTO{
			ID:        comment.UserID,
			Username:  uName,
			FullName:  uFullName,
			AvatarURL: uAvatarURL,
		},
		Replies:   make([]model.CommentDTO, 0),
		CreatedAt: comment.CreatedAt,
		UpdatedAt: comment.UpdatedAt,
	}, nil
}

func (r *commentRepository) UpdateComment(ctx context.Context, commentID, userID uuid.UUID, content string) (*model.CommentDTO, error) {
	query := `
		UPDATE post_comments 
		SET content = $1, is_edited = true, updated_at = NOW()
		WHERE id = $2 AND user_id = $3
		RETURNING post_slug, parent_id, is_edited, created_at, updated_at
	`
	var postSlug string
	var parentID *uuid.UUID
	var isEdited bool
	var createdAt, updatedAt time.Time

	err := r.db.Pool.QueryRow(ctx, query, content, commentID, userID).
		Scan(&postSlug, &parentID, &isEdited, &createdAt, &updatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to update comment: %w", err)
	}

	var uName, uFullName string
	var uAvatarURL *string
	userQuery := `SELECT username, full_name, avatar_url FROM users WHERE id = $1`
	if err := r.db.Pool.QueryRow(ctx, userQuery, userID).Scan(&uName, &uFullName, &uAvatarURL); err != nil {
		return nil, fmt.Errorf("failed to query author info: %w", err)
	}

	return &model.CommentDTO{
		ID:        commentID,
		PostSlug:  postSlug,
		ParentID:  parentID,
		Content:   content,
		IsEdited:  isEdited,
		IsAuthor:  true,
		Author: model.CommentAuthorDTO{
			ID:        userID,
			Username:  uName,
			FullName:  uFullName,
			AvatarURL: uAvatarURL,
		},
		Replies:   make([]model.CommentDTO, 0),
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}, nil
}

func (r *commentRepository) DeleteComment(ctx context.Context, commentID, userID uuid.UUID) error {
	query := `DELETE FROM post_comments WHERE id = $1 AND user_id = $2`
	res, err := r.db.Pool.Exec(ctx, query, commentID, userID)
	if err != nil {
		return fmt.Errorf("failed to delete comment: %w", err)
	}
	if res.RowsAffected() == 0 {
		return fmt.Errorf("comment not found or unauthorized")
	}
	return nil
}

func (r *commentRepository) GetCommentByID(ctx context.Context, commentID uuid.UUID) (*model.PostComment, error) {
	query := `
		SELECT id, post_slug, user_id, parent_id, content, is_edited, created_at, updated_at
		FROM post_comments
		WHERE id = $1
	`
	var c model.PostComment
	err := r.db.Pool.QueryRow(ctx, query, commentID).
		Scan(&c.ID, &c.PostSlug, &c.UserID, &c.ParentID, &c.Content, &c.IsEdited, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("comment not found: %w", err)
	}
	return &c, nil
}
