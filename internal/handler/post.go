package handler

import (
	"errors"
	"log"
	"net/http"

	"github.com/gofiber/fiber/v2"
	"github.com/irvanmalik48/realm-api/internal/config"
	"github.com/irvanmalik48/realm-api/internal/model"
	"github.com/irvanmalik48/realm-api/internal/repository"
	"github.com/irvanmalik48/realm-api/internal/service"
)

type PostHandler struct {
	cfg *config.Config
	svc service.PostService
}

func NewPostHandler(cfg *config.Config, svc service.PostService) *PostHandler {
	return &PostHandler{
		cfg: cfg,
		svc: svc,
	}
}

func (h *PostHandler) ListPosts(c *fiber.Ctx) error {
	tag := c.Query("tag")
	search := c.Query("search")
	limit := c.QueryInt("limit", 50)
	offset := c.QueryInt("offset", 0)

	var isPublishedOnly *bool
	if pub := c.Query("published"); pub != "" {
		val := pub == "true" || pub == "1"
		isPublishedOnly = &val
	}

	res, err := h.svc.ListPosts(c.Context(), isPublishedOnly, tag, search, limit, offset)
	if err != nil {
		log.Printf("[Posts] ListPosts error: %v\n", err)
		return ErrorResponse(c, "Failed to retrieve posts", http.StatusInternalServerError)
	}

	return JSONResponse(c, res, http.StatusOK, 0)
}

func (h *PostHandler) GetPost(c *fiber.Ctx) error {
	slug := c.Params("slug")
	if slug == "" {
		return ErrorResponse(c, "Post slug is required", http.StatusBadRequest)
	}

	// If query param include_drafts is true and user is authenticated admin, allow drafts
	isPublishedOnly := true
	if c.Query("include_drafts") == "true" {
		isPublishedOnly = false
	}

	post, err := h.svc.GetPostBySlug(c.Context(), slug, isPublishedOnly)
	if err != nil {
		if errors.Is(err, repository.ErrPostNotFound) {
			return ErrorResponse(c, "Post not found", http.StatusNotFound)
		}
		if errors.Is(err, service.ErrEmptySlug) {
			return ErrorResponse(c, err.Error(), http.StatusBadRequest)
		}
		log.Printf("[Posts] GetPost error for slug %q: %v\n", slug, err)
		return ErrorResponse(c, "Failed to retrieve post", http.StatusInternalServerError)
	}

	return JSONResponse(c, post, http.StatusOK, 0)
}

func (h *PostHandler) CreatePost(c *fiber.Ctx) error {
	var req model.CreatePostRequest
	if err := c.BodyParser(&req); err != nil {
		return ErrorResponse(c, "Invalid request payload", http.StatusBadRequest)
	}

	post, err := h.svc.CreatePost(c.Context(), &req)
	if err != nil {
		if errors.Is(err, service.ErrEmptyTitle) || errors.Is(err, service.ErrEmptySlug) {
			return ErrorResponse(c, err.Error(), http.StatusBadRequest)
		}
		log.Printf("[Posts] CreatePost error: %v\n", err)
		return ErrorResponse(c, "Failed to create post", http.StatusInternalServerError)
	}

	return JSONResponse(c, post, http.StatusCreated, 0)
}

func (h *PostHandler) UpdatePost(c *fiber.Ctx) error {
	slug := c.Params("slug")
	if slug == "" {
		return ErrorResponse(c, "Post slug is required", http.StatusBadRequest)
	}

	var req model.UpdatePostRequest
	if err := c.BodyParser(&req); err != nil {
		return ErrorResponse(c, "Invalid request payload", http.StatusBadRequest)
	}

	post, err := h.svc.UpdatePost(c.Context(), slug, &req)
	if err != nil {
		if errors.Is(err, repository.ErrPostNotFound) {
			return ErrorResponse(c, "Post not found", http.StatusNotFound)
		}
		if errors.Is(err, service.ErrEmptySlug) {
			return ErrorResponse(c, err.Error(), http.StatusBadRequest)
		}
		log.Printf("[Posts] UpdatePost error for slug %q: %v\n", slug, err)
		return ErrorResponse(c, "Failed to update post", http.StatusInternalServerError)
	}

	return JSONResponse(c, post, http.StatusOK, 0)
}

func (h *PostHandler) DeletePost(c *fiber.Ctx) error {
	slug := c.Params("slug")
	if slug == "" {
		return ErrorResponse(c, "Post slug is required", http.StatusBadRequest)
	}

	if err := h.svc.DeletePost(c.Context(), slug); err != nil {
		if errors.Is(err, repository.ErrPostNotFound) {
			return ErrorResponse(c, "Post not found", http.StatusNotFound)
		}
		if errors.Is(err, service.ErrEmptySlug) {
			return ErrorResponse(c, err.Error(), http.StatusBadRequest)
		}
		log.Printf("[Posts] DeletePost error for slug %q: %v\n", slug, err)
		return ErrorResponse(c, "Failed to delete post", http.StatusInternalServerError)
	}

	return JSONResponse(c, fiber.Map{"status": "success", "message": "Post deleted"}, http.StatusOK, 0)
}
