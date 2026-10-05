package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/irvanmalik48/realm-api/internal/model"
	"github.com/irvanmalik48/realm-api/internal/repository"
)

// AnalyticsService encapsulates business logic for analytics tracking and reporting.
type AnalyticsService struct {
	repo *repository.AnalyticsRepository
}

// NewAnalyticsService constructs an AnalyticsService.
func NewAnalyticsService(repo *repository.AnalyticsRepository) *AnalyticsService {
	return &AnalyticsService{repo: repo}
}

// TrackEvent processes an incoming pageview event and persists it.
func (s *AnalyticsService) TrackEvent(ctx context.Context, req *model.TrackEventRequest, clientIP string) error {
	path := strings.TrimSpace(req.Path)
	if path == "" {
		path = "/"
	}

	// Auto-extract post slug if visiting a blog post
	postSlug := strings.TrimSpace(req.PostSlug)
	if postSlug == "" && strings.HasPrefix(path, "/blog/") {
		trimmed := strings.TrimPrefix(path, "/blog/")
		parts := strings.Split(trimmed, "?")
		slugPart := strings.Trim(parts[0], "/")
		if slugPart != "" && !strings.Contains(slugPart, "/") {
			postSlug = slugPart
		}
	}

	browser, os, deviceType := parseUserAgent(req.UserAgent)

	ipHash := req.IPHash
	if ipHash == "" && clientIP != "" {
		h := sha256.Sum256([]byte(clientIP))
		ipHash = hex.EncodeToString(h[:16]) // 32 hex chars
	}

	pv := &model.PageView{
		Path:             path,
		Referrer:         strings.TrimSpace(req.Referrer),
		UserAgent:        strings.TrimSpace(req.UserAgent),
		Browser:          browser,
		OS:               os,
		DeviceType:       deviceType,
		SessionID:        strings.TrimSpace(req.SessionID),
		ScreenResolution: strings.TrimSpace(req.ScreenResolution),
		IPHash:           ipHash,
	}

	if postSlug != "" {
		pv.PostSlug = &postSlug
	}

	return s.repo.RecordPageView(ctx, pv)
}

// GetStats returns aggregated analytics over the requested time period.
func (s *AnalyticsService) GetStats(ctx context.Context, period string) (*model.AnalyticsStatsResponse, error) {
	return s.repo.GetAnalyticsStats(ctx, period)
}

// parseUserAgent extracts human-readable browser, OS, and device type.
func parseUserAgent(ua string) (browser, os, deviceType string) {
	if ua == "" {
		return "Unknown", "Unknown", "Desktop"
	}

	low := strings.ToLower(ua)

	// Device type
	if strings.Contains(low, "ipad") || strings.Contains(low, "tablet") {
		deviceType = "Tablet"
	} else if strings.Contains(low, "mobi") || strings.Contains(low, "android") || strings.Contains(low, "iphone") {
		deviceType = "Mobile"
	} else {
		deviceType = "Desktop"
	}

	// Operating System
	if strings.Contains(low, "windows") {
		os = "Windows"
	} else if strings.Contains(low, "macintosh") || strings.Contains(low, "mac os") {
		os = "macOS"
	} else if strings.Contains(low, "android") {
		os = "Android"
	} else if strings.Contains(low, "iphone") || strings.Contains(low, "ipad") {
		os = "iOS"
	} else if strings.Contains(low, "linux") {
		os = "Linux"
	} else {
		os = "Other"
	}

	// Browser
	if strings.Contains(low, "edg/") {
		browser = "Edge"
	} else if strings.Contains(low, "opr/") || strings.Contains(low, "opera") {
		browser = "Opera"
	} else if strings.Contains(low, "chrome") || strings.Contains(low, "crios") {
		browser = "Chrome"
	} else if strings.Contains(low, "firefox") || strings.Contains(low, "fxios") {
		browser = "Firefox"
	} else if strings.Contains(low, "safari") && !strings.Contains(low, "chrome") {
		browser = "Safari"
	} else {
		browser = "Other"
	}

	return browser, os, deviceType
}
