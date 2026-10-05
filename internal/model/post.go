package model

import (
	"time"

	"github.com/google/uuid"
)

// Post represents the database row for an article
type Post struct {
	ID          uuid.UUID `json:"id"`
	Slug        string    `json:"slug"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Content     string    `json:"content"`
	Tags        []string  `json:"tags"`
	CoverImage  *string   `json:"cover_image,omitempty"`
	ReadingTime string    `json:"reading_time"`
	IsPublished bool      `json:"is_published"`
	PublishedAt time.Time `json:"published_at"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// PostSummaryDTO represents the post without the full heavy markdown body
type PostSummaryDTO struct {
	ID          uuid.UUID `json:"id"`
	Slug        string    `json:"slug"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Tags        []string  `json:"tags"`
	CoverImage  *string   `json:"cover_image,omitempty"`
	ReadingTime string    `json:"reading_time"`
	IsPublished bool      `json:"is_published"`
	PublishedAt time.Time `json:"published_at"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// CreatePostRequest is the payload to create a new article
type CreatePostRequest struct {
	Slug        string     `json:"slug"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Content     string     `json:"content"`
	Tags        []string   `json:"tags"`
	CoverImage  *string    `json:"cover_image,omitempty"`
	IsPublished *bool      `json:"is_published,omitempty"`
	PublishedAt *time.Time `json:"published_at,omitempty"`
}

// UpdatePostRequest is the payload to modify an existing article
type UpdatePostRequest struct {
	Slug        *string    `json:"slug,omitempty"`
	Title       *string    `json:"title,omitempty"`
	Description *string    `json:"description,omitempty"`
	Content     *string    `json:"content,omitempty"`
	Tags        *[]string  `json:"tags,omitempty"`
	CoverImage  *string    `json:"cover_image,omitempty"`
	IsPublished *bool      `json:"is_published,omitempty"`
	PublishedAt *time.Time `json:"published_at,omitempty"`
}

// PostsListResponse is the response structure for listing posts
type PostsListResponse struct {
	Total int              `json:"total"`
	Posts []PostSummaryDTO `json:"posts"`
}
