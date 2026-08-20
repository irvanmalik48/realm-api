package storage

import (
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

type ImageInfo struct {
	Width    int
	Height   int
	Blurhash string
}

// IsImageMime returns true if the content type is an image format we can parse.
func IsImageMime(contentType string) bool {
	ct := strings.ToLower(strings.TrimSpace(contentType))
	return strings.HasPrefix(ct, "image/") && (strings.Contains(ct, "png") ||
		strings.Contains(ct, "jpeg") ||
		strings.Contains(ct, "jpg") ||
		strings.Contains(ct, "gif") ||
		strings.Contains(ct, "webp"))
}

// ProcessImage reads image data, computes width, height, and blurhash.
func ProcessImage(reader io.Reader) (*ImageInfo, error) {
	img, _, err := image.Decode(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("invalid image dimensions %dx%d", width, height)
	}

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

// EncodeToWebP decodes any supported image stream and writes it as WebP.
func EncodeToWebP(src io.Reader, dst io.Writer) error {
	img, _, err := image.Decode(src)
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
