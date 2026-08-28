package model

import (
	"time"

	"github.com/google/uuid"
)

// PostComment represents a comment record in the database
type PostComment struct {
	ID        uuid.UUID  `json:"id"`
	PostSlug  string     `json:"post_slug"`
	UserID    uuid.UUID  `json:"user_id"`
	ParentID  *uuid.UUID `json:"parent_id,omitempty"`
	Content   string     `json:"content"`
	IsEdited  bool       `json:"is_edited"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// CommentAuthorDTO contains public profile info for the comment author
type CommentAuthorDTO struct {
	ID        uuid.UUID `json:"id"`
	Username  string    `json:"username"`
	FullName  string    `json:"full_name"`
	AvatarURL *string   `json:"avatar_url,omitempty"`
}

// CommentDTO represents a comment formatted for frontend consumption
type CommentDTO struct {
	ID        uuid.UUID        `json:"id"`
	PostSlug  string           `json:"post_slug"`
	ParentID  *uuid.UUID       `json:"parent_id,omitempty"`
	Content   string           `json:"content"`
	IsEdited  bool             `json:"is_edited"`
	IsAuthor  bool             `json:"is_author"`
	Author    CommentAuthorDTO `json:"author"`
	Replies   []CommentDTO     `json:"replies"`
	CreatedAt time.Time        `json:"created_at"`
	UpdatedAt time.Time        `json:"updated_at"`
}

// CreateCommentRequest is the payload sent to create a new comment or reply
type CreateCommentRequest struct {
	Content  string     `json:"content"`
	ParentID *uuid.UUID `json:"parent_id,omitempty"`
}

// UpdateCommentRequest is the payload sent to edit an existing comment
type UpdateCommentRequest struct {
	Content string `json:"content"`
}

// CommentsListResponse is the response structure for listing comments on a post
type CommentsListResponse struct {
	Slug       string       `json:"slug"`
	TotalCount int          `json:"total_count"`
	Comments   []CommentDTO `json:"comments"`
}
