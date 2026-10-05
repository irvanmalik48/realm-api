package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/irvanmalik48/realm-api/internal/model"
	"github.com/irvanmalik48/realm-api/internal/repository"
)

var (
	ErrEmptyTitle   = errors.New("post title cannot be empty")
	ErrEmptyContent = errors.New("post content cannot be empty")
)

var slugRegex = regexp.MustCompile(`[^a-z0-9\-_]`)

type PostService interface {
	ListPosts(ctx context.Context, isPublishedOnly *bool, tag, search string, limit, offset int) (*model.PostsListResponse, error)
	GetPostBySlug(ctx context.Context, slug string, isPublishedOnly bool) (*model.Post, error)
	CreatePost(ctx context.Context, req *model.CreatePostRequest) (*model.Post, error)
	UpdatePost(ctx context.Context, slug string, req *model.UpdatePostRequest) (*model.Post, error)
	DeletePost(ctx context.Context, slug string) error
}

type postService struct {
	repo repository.PostRepository
}

func NewPostService(repo repository.PostRepository) PostService {
	return &postService{repo: repo}
}

func slugify(input string) string {
	s := strings.ToLower(strings.TrimSpace(input))
	s = strings.ReplaceAll(s, " ", "-")
	s = slugRegex.ReplaceAllString(s, "")
	s = strings.Trim(s, "-")
	return s
}

func (s *postService) ListPosts(ctx context.Context, isPublishedOnly *bool, tag, search string, limit, offset int) (*model.PostsListResponse, error) {
	return s.repo.ListPosts(ctx, isPublishedOnly, tag, search, limit, offset)
}

func (s *postService) GetPostBySlug(ctx context.Context, slug string, isPublishedOnly bool) (*model.Post, error) {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return nil, ErrEmptySlug
	}
	return s.repo.GetPostBySlug(ctx, slug, isPublishedOnly)
}

func (s *postService) CreatePost(ctx context.Context, req *model.CreatePostRequest) (*model.Post, error) {
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return nil, ErrEmptyTitle
	}

	slug := strings.TrimSpace(req.Slug)
	if slug == "" {
		slug = slugify(title)
	} else {
		slug = slugify(slug)
	}
	if slug == "" {
		return nil, ErrEmptySlug
	}

	content := strings.TrimSpace(req.Content)

	words := len(strings.Fields(content))
	mins := (words + 119) / 120
	if mins < 1 {
		mins = 1
	}
	readingTime := fmt.Sprintf("%d min read", mins)

	isPublished := true
	if req.IsPublished != nil {
		isPublished = *req.IsPublished
	}

	publishedAt := time.Now()
	if req.PublishedAt != nil && !req.PublishedAt.IsZero() {
		publishedAt = *req.PublishedAt
	}

	tags := req.Tags
	if tags == nil {
		tags = []string{}
	}

	post := &model.Post{
		Slug:        slug,
		Title:       title,
		Description: strings.TrimSpace(req.Description),
		Content:     content,
		Tags:        tags,
		CoverImage:  req.CoverImage,
		ReadingTime: readingTime,
		IsPublished: isPublished,
		PublishedAt: publishedAt,
	}

	return s.repo.CreatePost(ctx, post)
}

func (s *postService) UpdatePost(ctx context.Context, slug string, req *model.UpdatePostRequest) (*model.Post, error) {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return nil, ErrEmptySlug
	}

	if req.Slug != nil {
		newSlug := slugify(*req.Slug)
		if newSlug != "" {
			req.Slug = &newSlug
		}
	}

	return s.repo.UpdatePost(ctx, slug, req)
}

func (s *postService) DeletePost(ctx context.Context, slug string) error {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return ErrEmptySlug
	}
	return s.repo.DeletePost(ctx, slug)
}
