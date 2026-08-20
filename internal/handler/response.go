package handler

import (
	"fmt"

	"github.com/gofiber/fiber/v2"
)

type ErrorResponsePayload struct {
	Error  string `json:"error"`
	Status string `json:"status"`
}

type RootResponsePayload struct {
	Message string `json:"message"`
	Status  string `json:"status"`
}

func JSONResponse(c *fiber.Ctx, body interface{}, status int, revalidateSeconds int) error {
	if revalidateSeconds > 0 {
		c.Set("Cache-Control", fmt.Sprintf("public, s-maxage=%d, stale-while-revalidate=%d", revalidateSeconds, revalidateSeconds*2))
	}
	return c.Status(status).JSON(body)
}

func ErrorResponse(c *fiber.Ctx, message string, status int) error {
	return JSONResponse(c, ErrorResponsePayload{
		Error:  message,
		Status: "error",
	}, status, 0)
}
