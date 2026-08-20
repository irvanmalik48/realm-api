package handler

import (
	"log"
	"net/http"
	"regexp"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/irvanmalik48/realm-api/internal/config"
	"github.com/irvanmalik48/realm-api/internal/model"
	"github.com/irvanmalik48/realm-api/internal/service"
)

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

type ContactHandler struct {
	cfg     *config.Config
	service service.ContactService
}

func NewContactHandler(cfg *config.Config, svc service.ContactService) *ContactHandler {
	return &ContactHandler{
		cfg:     cfg,
		service: svc,
	}
}

func (h *ContactHandler) Handle(c *fiber.Ctx) error {
	var req model.ContactRequest
	if err := c.BodyParser(&req); err != nil {
		return ErrorResponse(c, "Invalid JSON payload", http.StatusBadRequest)
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Email = strings.TrimSpace(req.Email)
	req.Subject = strings.TrimSpace(req.Subject)
	req.Message = strings.TrimSpace(req.Message)

	if req.Name == "" {
		return ErrorResponse(c, "Name is required", http.StatusBadRequest)
	}
	if len(req.Name) < 2 {
		return ErrorResponse(c, "Name must be at least 2 characters", http.StatusBadRequest)
	}
	if len(req.Name) > 100 {
		return ErrorResponse(c, "Name cannot exceed 100 characters", http.StatusBadRequest)
	}

	if req.Email == "" {
		return ErrorResponse(c, "Email is required", http.StatusBadRequest)
	}
	if !emailRegex.MatchString(req.Email) || len(req.Email) > 254 {
		return ErrorResponse(c, "Invalid email address", http.StatusBadRequest)
	}

	if req.Subject == "" {
		return ErrorResponse(c, "Subject is required", http.StatusBadRequest)
	}
	if len(req.Subject) < 3 {
		return ErrorResponse(c, "Subject must be at least 3 characters", http.StatusBadRequest)
	}
	if len(req.Subject) > 200 {
		return ErrorResponse(c, "Subject cannot exceed 200 characters", http.StatusBadRequest)
	}

	if req.Message == "" {
		return ErrorResponse(c, "Message is required", http.StatusBadRequest)
	}
	if len(req.Message) < 10 {
		return ErrorResponse(c, "Message must be at least 10 characters", http.StatusBadRequest)
	}
	if len(req.Message) > 5000 {
		return ErrorResponse(c, "Message cannot exceed 5000 characters", http.StatusBadRequest)
	}

	ipAddress := c.IP()
	userAgent := c.Get("User-Agent")

	if _, err := h.service.SendMessage(c.Context(), &req, ipAddress, userAgent); err != nil {
		log.Printf("Failed to process contact message: %v\n", err)
		return ErrorResponse(c, "Failed to save message", http.StatusInternalServerError)
	}

	return c.Status(http.StatusOK).JSON(model.ContactResponse{
		Message: "Your message has been sent successfully.",
		Status:  "success",
	})
}
