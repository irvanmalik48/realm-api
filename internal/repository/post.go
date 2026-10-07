package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/irvanmalik48/realm-api/internal/database"
	"github.com/irvanmalik48/realm-api/internal/model"
)

var ErrPostNotFound = errors.New("post not found")

type PostRepository interface {
	ListPosts(ctx context.Context, isPublishedOnly *bool, tag, search string, limit, offset int) (*model.PostsListResponse, error)
	GetPostBySlug(ctx context.Context, slug string, isPublishedOnly bool) (*model.Post, error)
	CreatePost(ctx context.Context, post *model.Post) (*model.Post, error)
	UpdatePost(ctx context.Context, slug string, req *model.UpdatePostRequest) (*model.Post, error)
	DeletePost(ctx context.Context, slug string) error
}

type postRepository struct {
	db *database.DB
}

func NewPostRepository(db *database.DB) PostRepository {
	return &postRepository{db: db}
}

func (r *postRepository) ListPosts(ctx context.Context, isPublishedOnly *bool, tag, search string, limit, offset int) (*model.PostsListResponse, error) {
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	var conditions []string
	var args []interface{}
	argIdx := 1

	if isPublishedOnly != nil && *isPublishedOnly {
		conditions = append(conditions, "is_published = true")
	}

	if tag != "" {
		conditions = append(conditions, fmt.Sprintf("$%d = ANY(tags)", argIdx))
		args = append(args, tag)
		argIdx++
	}

	if search != "" {
		conditions = append(conditions, fmt.Sprintf("(title ILIKE $%d OR description ILIKE $%d OR slug ILIKE $%d)", argIdx, argIdx, argIdx))
		args = append(args, "%"+search+"%")
		argIdx++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// Count query
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM posts %s", whereClause)
	var total int
	err := r.db.Pool.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, fmt.Errorf("failed to count posts: %w", err)
	}

	// Data query
	dataQuery := fmt.Sprintf(`
		SELECT 
			id, slug, title, description, tags, cover_image, reading_time, is_published, published_at, created_at, updated_at
		FROM posts
		%s
		ORDER BY COALESCE(updated_at, published_at, created_at) DESC, created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIdx, argIdx+1)

	args = append(args, limit, offset)

	rows, err := r.db.Pool.Query(ctx, dataQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list posts: %w", err)
	}
	defer rows.Close()

	posts := make([]model.PostSummaryDTO, 0)
	for rows.Next() {
		var p model.PostSummaryDTO
		if err := rows.Scan(
			&p.ID, &p.Slug, &p.Title, &p.Description, &p.Tags, &p.CoverImage,
			&p.ReadingTime, &p.IsPublished, &p.PublishedAt, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan post summary: %w", err)
		}
		if p.Tags == nil {
			p.Tags = []string{}
		}
		posts = append(posts, p)
	}

	return &model.PostsListResponse{
		Total: total,
		Posts: posts,
	}, nil
}

func (r *postRepository) GetPostBySlug(ctx context.Context, slug string, isPublishedOnly bool) (*model.Post, error) {
	query := `
		SELECT 
			id, slug, title, description, content, tags, cover_image, reading_time, is_published, published_at, created_at, updated_at
		FROM posts
		WHERE slug = $1
	`
	if isPublishedOnly {
		query += " AND is_published = true"
	}

	var p model.Post
	err := r.db.Pool.QueryRow(ctx, query, slug).Scan(
		&p.ID, &p.Slug, &p.Title, &p.Description, &p.Content, &p.Tags, &p.CoverImage,
		&p.ReadingTime, &p.IsPublished, &p.PublishedAt, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrPostNotFound
		}
		return nil, fmt.Errorf("failed to get post by slug: %w", err)
	}
	if p.Tags == nil {
		p.Tags = []string{}
	}

	return &p, nil
}

func (r *postRepository) CreatePost(ctx context.Context, post *model.Post) (*model.Post, error) {
	query := `
		INSERT INTO posts (slug, title, description, content, tags, cover_image, reading_time, is_published, published_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, slug, title, description, content, tags, cover_image, reading_time, is_published, published_at, created_at, updated_at
	`
	var p model.Post
	err := r.db.Pool.QueryRow(ctx, query,
		post.Slug, post.Title, post.Description, post.Content, post.Tags, post.CoverImage,
		post.ReadingTime, post.IsPublished, post.PublishedAt,
	).Scan(
		&p.ID, &p.Slug, &p.Title, &p.Description, &p.Content, &p.Tags, &p.CoverImage,
		&p.ReadingTime, &p.IsPublished, &p.PublishedAt, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create post: %w", err)
	}
	if p.Tags == nil {
		p.Tags = []string{}
	}

	return &p, nil
}

func (r *postRepository) UpdatePost(ctx context.Context, slug string, req *model.UpdatePostRequest) (*model.Post, error) {
	var sets []string
	var args []interface{}
	argIdx := 1

	if req.Slug != nil {
		sets = append(sets, fmt.Sprintf("slug = $%d", argIdx))
		args = append(args, *req.Slug)
		argIdx++
	}
	if req.Title != nil {
		sets = append(sets, fmt.Sprintf("title = $%d", argIdx))
		args = append(args, *req.Title)
		argIdx++
	}
	if req.Description != nil {
		sets = append(sets, fmt.Sprintf("description = $%d", argIdx))
		args = append(args, *req.Description)
		argIdx++
	}
	if req.Content != nil {
		sets = append(sets, fmt.Sprintf("content = $%d", argIdx))
		args = append(args, *req.Content)
		argIdx++
		// Recompute reading time
		words := len(strings.Fields(*req.Content))
		mins := (words + 119) / 120
		if mins < 1 {
			mins = 1
		}
		sets = append(sets, fmt.Sprintf("reading_time = $%d", argIdx))
		args = append(args, fmt.Sprintf("%d min read", mins))
		argIdx++
	}
	if req.Tags != nil {
		sets = append(sets, fmt.Sprintf("tags = $%d", argIdx))
		args = append(args, *req.Tags)
		argIdx++
	}
	if req.CoverImage != nil {
		sets = append(sets, fmt.Sprintf("cover_image = $%d", argIdx))
		args = append(args, req.CoverImage)
		argIdx++
	}
	if req.IsPublished != nil {
		sets = append(sets, fmt.Sprintf("is_published = $%d", argIdx))
		args = append(args, *req.IsPublished)
		argIdx++
	}
	if req.PublishedAt != nil {
		sets = append(sets, fmt.Sprintf("published_at = $%d", argIdx))
		args = append(args, *req.PublishedAt)
		argIdx++
	}

	sets = append(sets, "updated_at = NOW()")

	if len(sets) == 1 { // only updated_at
		return r.GetPostBySlug(ctx, slug, false)
	}

	query := fmt.Sprintf(`
		UPDATE posts
		SET %s
		WHERE slug = $%d
		RETURNING id, slug, title, description, content, tags, cover_image, reading_time, is_published, published_at, created_at, updated_at
	`, strings.Join(sets, ", "), argIdx)

	args = append(args, slug)

	var p model.Post
	err := r.db.Pool.QueryRow(ctx, query, args...).Scan(
		&p.ID, &p.Slug, &p.Title, &p.Description, &p.Content, &p.Tags, &p.CoverImage,
		&p.ReadingTime, &p.IsPublished, &p.PublishedAt, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrPostNotFound
		}
		return nil, fmt.Errorf("failed to update post: %w", err)
	}
	if p.Tags == nil {
		p.Tags = []string{}
	}

	return &p, nil
}

func (r *postRepository) DeletePost(ctx context.Context, slug string) error {
	query := "DELETE FROM posts WHERE slug = $1"
	tag, err := r.db.Pool.Exec(ctx, query, slug)
	if err != nil {
		return fmt.Errorf("failed to delete post: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrPostNotFound
	}
	return nil
}
