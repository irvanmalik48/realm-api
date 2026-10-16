package model

import (
	"time"

	"github.com/google/uuid"
)

// PostComment represents a comment record in the database
type PostComment struct {
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	ParentID  *uuid.UUID `json:"parent_id,omitempty"`
	PostSlug  string     `json:"post_slug"`
	Content   string     `json:"content"`
	ID        uuid.UUID  `json:"id"`
	UserID    uuid.UUID  `json:"user_id"`
	IsEdited  bool       `json:"is_edited"`
}

// CommentAuthorDTO contains public profile info for the comment author
type CommentAuthorDTO struct {
	AvatarURL *string   `json:"avatar_url,omitempty"`
	Username  string    `json:"username"`
	FullName  string    `json:"full_name"`
	ID        uuid.UUID `json:"id"`
}

// CommentDTO represents a comment formatted for frontend consumption
type CommentDTO struct {
	CreatedAt time.Time        `json:"created_at"`
	UpdatedAt time.Time        `json:"updated_at"`
	ParentID  *uuid.UUID       `json:"parent_id,omitempty"`
	PostSlug  string           `json:"post_slug"`
	Content   string           `json:"content"`
	Replies   []CommentDTO     `json:"replies"`
	Author    CommentAuthorDTO `json:"author"`
	ID        uuid.UUID        `json:"id"`
	IsEdited  bool             `json:"is_edited"`
	IsAuthor  bool             `json:"is_author"`
}

// CreateCommentRequest is the payload sent to create a new comment or reply
type CreateCommentRequest struct {
	ParentID *uuid.UUID `json:"parent_id,omitempty"`
	Content  string     `json:"content"`
}

// UpdateCommentRequest is the payload sent to edit an existing comment
type UpdateCommentRequest struct {
	Content string `json:"content"`
}

// CommentsListResponse is the response structure for listing comments on a post
type CommentsListResponse struct {
	Slug       string       `json:"slug"`
	Comments   []CommentDTO `json:"comments"`
	TotalCount int          `json:"total_count"`
}
