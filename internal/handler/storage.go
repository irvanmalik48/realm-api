package handler

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/irvanmalik48/realm-api/internal/config"
	"github.com/irvanmalik48/realm-api/internal/model"
	"github.com/irvanmalik48/realm-api/internal/repository"
	"github.com/irvanmalik48/realm-api/internal/service"
	"github.com/irvanmalik48/realm-api/internal/storage"
)

type StorageHandler struct {
	cfg     *config.Config
	service service.StorageService
}

func NewStorageHandler(cfg *config.Config, svc service.StorageService) *StorageHandler {
	return &StorageHandler{
		cfg:     cfg,
		service: svc,
	}
}

func (h *StorageHandler) Upload(c *fiber.Ctx) error {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		return ErrorResponse(c, "No file uploaded. Please provide a 'file' form field.", http.StatusBadRequest)
	}

	srcFile, err := fileHeader.Open()
	if err != nil {
		return ErrorResponse(c, "Failed to open uploaded file", http.StatusBadRequest)
	}
	defer srcFile.Close()

	explicitType := fileHeader.Header.Get("Content-Type")
	fileDTO, err := h.service.Upload(c.Context(), fileHeader.Filename, srcFile, explicitType)
	if err != nil {
		if errors.Is(err, service.ErrFileTooLarge) {
			return ErrorResponse(c, err.Error(), http.StatusRequestEntityTooLarge)
		}
		return ErrorResponse(c, fmt.Sprintf("Failed to process file: %v", err), http.StatusInternalServerError)
	}

	return c.Status(http.StatusCreated).JSON(model.FileUploadResponse{
		Status:  "success",
		Message: "File uploaded and compressed successfully",
		File:    *fileDTO,
	})
}

func (h *StorageHandler) GetFile(c *fiber.Ctx) error {
	idParam := c.Params("id")
	fileID, err := uuid.Parse(idParam)
	if err != nil {
		return ErrorResponse(c, "Invalid file UUID", http.StatusBadRequest)
	}

	formatParam := strings.ToLower(c.Query("format"))
	acceptHeader := strings.ToLower(c.Get("Accept"))
	requestWebP := formatParam == "webp" || c.Query("webp") == "true" || c.Query("webp") == "1" ||
		(formatParam == "" && strings.Contains(acceptHeader, "image/webp") && !strings.Contains(acceptHeader, "image/*") && !strings.Contains(acceptHeader, "*/*"))

	var record *model.FileRecord
	var stream io.Reader

	if requestWebP {
		rec, rdr, err := h.service.GetAsWebP(c.Context(), fileID)
		if err == nil {
			record = rec
			stream = rdr
		} else if errors.Is(err, repository.ErrRecordNotFound) || errors.Is(err, storage.ErrFileNotFound) {
			return ErrorResponse(c, "File not found", http.StatusNotFound)
		} else if !errors.Is(err, service.ErrUnsupportedFormat) {
			return ErrorResponse(c, fmt.Sprintf("Failed to convert image: %v", err), http.StatusInternalServerError)
		}
	}

	if record == nil {
		rec, readCloser, err := h.service.Get(c.Context(), fileID)
		if err != nil {
			if errors.Is(err, repository.ErrRecordNotFound) || errors.Is(err, storage.ErrFileNotFound) {
				return ErrorResponse(c, "File not found", http.StatusNotFound)
			}
			return ErrorResponse(c, fmt.Sprintf("Failed to retrieve file: %v", err), http.StatusInternalServerError)
		}
		record = rec
		stream = readCloser
		requestWebP = false
	}

	// Caching & ETag
	etag := fmt.Sprintf(`"%s"`, record.SHA256)
	if requestWebP {
		etag = fmt.Sprintf(`"webp-%s"`, record.SHA256)
	}

	if match := c.Get("If-None-Match"); match != "" && (match == etag || match == "*") {
		return c.SendStatus(http.StatusNotModified)
	}

	c.Set("Content-Type", record.ContentType)
	c.Set("ETag", etag)
	c.Set("Cache-Control", "public, max-age=31536000, immutable")

	// Disposition
	isDownload := c.Query("download") == "true" || c.Query("download") == "1"
	dispositionType := "inline"
	if isDownload {
		dispositionType = "attachment"
	}
	c.Set("Content-Disposition", fmt.Sprintf(`%s; filename="%s"`, dispositionType, record.Filename))

	// Image metadata headers
	if record.Blurhash != nil && *record.Blurhash != "" {
		c.Set("X-Blurhash", *record.Blurhash)
	}
	if record.Width != nil && *record.Width > 0 {
		c.Set("X-Image-Width", strconv.Itoa(*record.Width))
	}
	if record.Height != nil && *record.Height > 0 {
		c.Set("X-Image-Height", strconv.Itoa(*record.Height))
	}

	return c.SendStream(stream)
}

func (h *StorageHandler) GetFileInfo(c *fiber.Ctx) error {
	idParam := c.Params("id")
	fileID, err := uuid.Parse(idParam)
	if err != nil {
		return ErrorResponse(c, "Invalid file UUID", http.StatusBadRequest)
	}

	dto, err := h.service.GetInfo(c.Context(), fileID)
	if err != nil {
		if errors.Is(err, repository.ErrRecordNotFound) {
			return ErrorResponse(c, "File not found", http.StatusNotFound)
		}
		return ErrorResponse(c, fmt.Sprintf("Failed to get file info: %v", err), http.StatusInternalServerError)
	}

	return c.Status(http.StatusOK).JSON(model.FileInfoResponse{
		Status: "success",
		File:   *dto,
	})
}

func (h *StorageHandler) DeleteFile(c *fiber.Ctx) error {
	idParam := c.Params("id")
	fileID, err := uuid.Parse(idParam)
	if err != nil {
		return ErrorResponse(c, "Invalid file UUID", http.StatusBadRequest)
	}

	if err := h.service.Delete(c.Context(), fileID); err != nil {
		if errors.Is(err, repository.ErrRecordNotFound) || errors.Is(err, storage.ErrFileNotFound) {
			return ErrorResponse(c, "File not found", http.StatusNotFound)
		}
		return ErrorResponse(c, fmt.Sprintf("Failed to delete file: %v", err), http.StatusInternalServerError)
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"message": "File deleted successfully",
		"status":  "success",
	})
}
