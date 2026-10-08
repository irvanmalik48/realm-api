package storage

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/HugoSmits86/nativewebp"
	"github.com/bbrks/go-blurhash"
	"golang.org/x/image/draw"
)

const (
	MaxFetchImageSize = 15 * 1024 * 1024 // 15MB limit for external fetches
	FetchTimeout      = 10 * time.Second
)

var (
	ErrInvalidURL       = errors.New("invalid image URL")
	ErrSSRFForbidden    = errors.New("access to private, local, or loopback network addresses is forbidden")
	ErrFetchImageFailed = errors.New("failed to fetch image from URL")
)

// ImageMetadata represents the analyzed properties of an image formatted for Next/Image compatibility.
type ImageMetadata struct {
	Src         string  `json:"src"`
	Width       int     `json:"width"`
	Height      int     `json:"height"`
	AspectRatio float64 `json:"aspectRatio"`
	Format      string  `json:"format"`
	Size        int64   `json:"size"`
	Blurhash    string  `json:"blurhash"`
	BlurDataURL string  `json:"blurDataURL"`
	BlurWidth   int     `json:"blurWidth,omitempty"`
	BlurHeight  int     `json:"blurHeight,omitempty"`
}

// isPrivateOrLocalIP checks whether an IP address is private, loopback, link-local, or unspecified.
func isPrivateOrLocalIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
		return true
	}
	// Also check IPv4-mapped IPv6 addresses (e.g. ::ffff:127.0.0.1)
	if ip4 := ip.To4(); ip4 != nil {
		if ip4.IsLoopback() || ip4.IsPrivate() || ip4.IsLinkLocalUnicast() || ip4.IsLinkLocalMulticast() || ip4.IsUnspecified() {
			return true
		}
	}
	return false
}

// ValidateImageURL validates the URL scheme, host, and guards against SSRF.
func ValidateImageURL(rawURL string) (*url.URL, error) {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return nil, fmt.Errorf("%w: URL cannot be empty", ErrInvalidURL)
	}

	parsed, err := url.ParseRequestURI(trimmed)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidURL, err)
	}

	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return nil, fmt.Errorf("%w: only http and https schemes are supported", ErrInvalidURL)
	}

	hostname := parsed.Hostname()
	if hostname == "" {
		return nil, fmt.Errorf("%w: missing hostname", ErrInvalidURL)
	}

	// Resolve host IPs to guard against internal/private infrastructure probing (SSRF)
	ips, err := net.LookupIP(hostname)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve host %q: %w", hostname, err)
	}

	for _, ip := range ips {
		if isPrivateOrLocalIP(ip) {
			return nil, fmt.Errorf("%w: host %q resolves to restricted IP %s", ErrSSRFForbidden, hostname, ip.String())
		}
	}

	return parsed, nil
}

// FetchAndProcessImage downloads an image safely and computes its metadata, blurhash, and blurDataURL.
func FetchAndProcessImage(ctx context.Context, targetURL string) (*ImageMetadata, error) {
	parsedURL, err := ValidateImageURL(targetURL)
	if err != nil {
		return nil, err
	}

	dialer := &net.Dialer{
		Timeout: 5 * time.Second,
	}

	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}

			ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve host %q: %w", host, err)
			}

			var safeIP net.IP
			for _, ip := range ips {
				if !isPrivateOrLocalIP(ip) {
					safeIP = ip
					break
				}
			}

			if safeIP == nil {
				return nil, fmt.Errorf("%w: host %q resolved only to restricted addresses", ErrSSRFForbidden, host)
			}

			return dialer.DialContext(ctx, network, net.JoinHostPort(safeIP.String(), port))
		},
		ResponseHeaderTimeout: 5 * time.Second,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   FetchTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("too many redirects (max 5)")
			}
			if _, err := ValidateImageURL(req.URL.String()); err != nil {
				return err
			}
			return nil
		},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsedURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}
	req.Header.Set("User-Agent", "Realm-Image-Bot/1.0 (+https://realm.irvanmalik48.com)")
	req.Header.Set("Accept", "image/avif,image/webp,image/apng,image/svg+xml,image/*,*/*;q=0.8")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrFetchImageFailed, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%w: server returned status %d", ErrFetchImageFailed, resp.StatusCode)
	}

	// Guard against payload bombs with LimitReader
	limitReader := io.LimitReader(resp.Body, MaxFetchImageSize+1)
	data, err := io.ReadAll(limitReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read image body: %w", err)
	}
	if int64(len(data)) > MaxFetchImageSize {
		return nil, fmt.Errorf("image exceeds maximum allowed size of %d bytes", MaxFetchImageSize)
	}

	return ProcessImageBuffer(data, targetURL)
}

// ProcessImageBuffer decodes an in-memory image buffer, computes blurhash and generates a small blurDataURL.
func ProcessImageBuffer(data []byte, srcURL string) (*ImageMetadata, error) {
	if len(data) == 0 {
		return nil, errors.New("empty image data buffer")
	}

	// 1. Inspect image dimensions and format safely without allocating full pixel memory
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to decode image config: %w", err)
	}

	if cfg.Width <= 0 || cfg.Height <= 0 {
		return nil, fmt.Errorf("invalid image dimensions: %dx%d", cfg.Width, cfg.Height)
	}

	if cfg.Width > MaxImageDimension || cfg.Height > MaxImageDimension || (cfg.Width*cfg.Height) > MaxImagePixels {
		return nil, fmt.Errorf("%w: %dx%d", ErrImageTooLarge, cfg.Width, cfg.Height)
	}

	// 2. Decode pixels now that safety limits have passed
	img, detectedFormat, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}
	if detectedFormat != "" {
		format = detectedFormat
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	aspectRatio := math.Round((float64(width)/float64(height))*1000) / 1000

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

	// Generate proportional low-resolution placeholder (width = 16px)
	targetW := 16
	targetH := int(math.Round(float64(targetW) * float64(height) / float64(width)))
	if targetH < 1 {
		targetH = 1
	} else if targetH > 32 {
		targetH = 32
	}

	dst := image.NewRGBA(image.Rect(0, 0, targetW, targetH))
	draw.BiLinear.Scale(dst, dst.Bounds(), img, bounds, draw.Over, nil)

	var blurBuf bytes.Buffer
	var blurDataURL string

	opts := &nativewebp.Options{
		CompressionLevel: nativewebp.DefaultCompression,
	}
	if err := nativewebp.Encode(&blurBuf, dst, opts); err == nil && blurBuf.Len() > 0 {
		blurDataURL = "data:image/webp;base64," + base64.StdEncoding.EncodeToString(blurBuf.Bytes())
	} else {
		// Fallback to PNG placeholder if WebP encoding produces no output
		blurBuf.Reset()
		if err := png.Encode(&blurBuf, dst); err == nil {
			blurDataURL = "data:image/png;base64," + base64.StdEncoding.EncodeToString(blurBuf.Bytes())
		}
	}

	return &ImageMetadata{
		Src:         srcURL,
		Width:       width,
		Height:      height,
		AspectRatio: aspectRatio,
		Format:      format,
		Size:        int64(len(data)),
		Blurhash:    bh,
		BlurDataURL: blurDataURL,
		BlurWidth:   targetW,
		BlurHeight:  targetH,
	}, nil
}
