package storage

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
)

func createTestPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x % 256), G: uint8(y % 256), B: 128, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("failed to encode test png: %v", err)
	}
	return buf.Bytes()
}

func TestProcessImageBuffer(t *testing.T) {
	data := createTestPNG(t, 200, 100)
	meta, err := ProcessImageBuffer(data, "https://example.com/test.png")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if meta.Width != 200 {
		t.Errorf("expected width 200, got %d", meta.Width)
	}
	if meta.Height != 100 {
		t.Errorf("expected height 100, got %d", meta.Height)
	}
	if meta.AspectRatio != 2.0 {
		t.Errorf("expected aspect ratio 2.0, got %f", meta.AspectRatio)
	}
	if meta.Format != "png" {
		t.Errorf("expected format png, got %s", meta.Format)
	}
	if meta.Blurhash == "" {
		t.Errorf("expected non-empty blurhash")
	}
	if !strings.HasPrefix(meta.BlurDataURL, "data:image/") {
		t.Errorf("expected blurDataURL to start with data:image/, got %q", meta.BlurDataURL)
	}
	if meta.BlurWidth != 16 {
		t.Errorf("expected blurWidth 16, got %d", meta.BlurWidth)
	}
	if meta.BlurHeight != 8 {
		t.Errorf("expected blurHeight 8, got %d", meta.BlurHeight)
	}
}

func TestValidateImageURL_SSRF(t *testing.T) {
	tests := []struct {
		url     string
		wantErr bool
	}{
		{"http://127.0.0.1/test.png", true},
		{"http://localhost/test.png", true},
		{"http://10.0.0.1/test.png", true},
		{"http://192.168.1.1/test.png", true},
		{"http://169.254.169.254/latest/meta-data/", true},
		{"ftp://example.com/image.png", true},
		{"file:///etc/passwd", true},
		{"", true},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			_, err := ValidateImageURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateImageURL(%q) err = %v, wantErr = %v", tt.url, err, tt.wantErr)
			}
		})
	}
}
