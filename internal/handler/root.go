package handler

import (
	"net/http"

	"github.com/gofiber/fiber/v2"
)

type RootHandler struct{}

func NewRootHandler() *RootHandler {
	return &RootHandler{}
}

func (h *RootHandler) Handle(c *fiber.Ctx) error {
	return c.Status(http.StatusOK).JSON(RootResponsePayload{
		Message: "Nothing to see here",
		Status:  "success",
	})
}
