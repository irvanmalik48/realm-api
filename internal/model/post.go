package model

import (
	"time"

	"github.com/google/uuid"
)

// Post represents the database row for an article
type Post struct {
	PublishedAt time.Time `json:"published_at"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	CoverImage  *string   `json:"cover_image,omitempty"`
	Slug        string    `json:"slug"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Content     string    `json:"content"`
	ReadingTime string    `json:"reading_time"`
	Tags        []string  `json:"tags"`
	ID          uuid.UUID `json:"id"`
	IsPublished bool      `json:"is_published"`
}

// PostSummaryDTO represents the post without the full heavy markdown body
type PostSummaryDTO struct {
	PublishedAt time.Time `json:"published_at"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	CoverImage  *string   `json:"cover_image,omitempty"`
	Slug        string    `json:"slug"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	ReadingTime string    `json:"reading_time"`
	Tags        []string  `json:"tags"`
	ID          uuid.UUID `json:"id"`
	IsPublished bool      `json:"is_published"`
}

// CreatePostRequest is the payload to create a new article
type CreatePostRequest struct {
	CoverImage  *string    `json:"cover_image,omitempty"`
	IsPublished *bool      `json:"is_published,omitempty"`
	PublishedAt *time.Time `json:"published_at,omitempty"`
	Slug        string     `json:"slug"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Content     string     `json:"content"`
	Tags        []string   `json:"tags"`
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
	Posts []PostSummaryDTO `json:"posts"`
	Total int              `json:"total"`
}
