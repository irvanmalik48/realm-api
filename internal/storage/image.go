package storage

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"strings"

	"github.com/HugoSmits86/nativewebp"
	"github.com/bbrks/go-blurhash"
	_ "golang.org/x/image/webp"
)

const (
	MaxImageDimension = 4096
	MaxImagePixels    = 4096 * 4096 // 16.7 megapixels max safety limit
)

var (
	ErrImageTooLarge = errors.New("image dimensions exceed maximum allowed dimensions (max 4096x4096)")
)

type ImageInfo struct {
	Width    int
	Height   int
	Blurhash string
}

// IsImageMime returns true if the content type is a safe image format we can parse.
func IsImageMime(contentType string) bool {
	ct := strings.ToLower(strings.TrimSpace(contentType))
	return ct == "image/png" ||
		ct == "image/jpeg" ||
		ct == "image/jpg" ||
		ct == "image/gif" ||
		ct == "image/webp"
}

// ProcessImage reads image data, checks dimension limits to prevent decompression bombs, and computes blurhash.
func ProcessImage(reader io.Reader) (*ImageInfo, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read image data: %w", err)
	}

	// 1. Inspect image dimensions safely without allocating pixel memory
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to decode image config: %w", err)
	}

	if cfg.Width <= 0 || cfg.Height <= 0 {
		return nil, fmt.Errorf("invalid image dimensions %dx%d", cfg.Width, cfg.Height)
	}

	if cfg.Width > MaxImageDimension || cfg.Height > MaxImageDimension || (cfg.Width*cfg.Height) > MaxImagePixels {
		return nil, fmt.Errorf("%w: %dx%d", ErrImageTooLarge, cfg.Width, cfg.Height)
	}

	// 2. Decode pixels now that dimensions are verified safe
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// Choose blurhash component x and y based on aspect ratio
	xComp := 4
	yComp := 3
	if height > width {
		xComp = 3
		yComp = 4
	}

	bh, err := blurhash.Encode(xComp, yComp, img)
	if err != nil {
		return nil, fmt.Errorf("failed to compute blurhash: %w", err)
	}

	return &ImageInfo{
		Width:    width,
		Height:   height,
		Blurhash: bh,
	}, nil
}

// EncodeToWebP safely decodes any supported image stream with dimension guards and writes it as WebP.
func EncodeToWebP(src io.Reader, dst io.Writer) error {
	data, err := io.ReadAll(src)
	if err != nil {
		return fmt.Errorf("failed to read image data: %w", err)
	}

	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to decode source image config: %w", err)
	}

	if cfg.Width > MaxImageDimension || cfg.Height > MaxImageDimension || (cfg.Width*cfg.Height) > MaxImagePixels {
		return fmt.Errorf("%w: %dx%d", ErrImageTooLarge, cfg.Width, cfg.Height)
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to decode source image for webp conversion: %w", err)
	}

	opts := &nativewebp.Options{
		CompressionLevel: nativewebp.DefaultCompression,
	}

	if err := nativewebp.Encode(dst, img, opts); err != nil {
		return fmt.Errorf("failed to encode image to webp: %w", err)
	}

	return nil
}
