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

	site := normalizeSite(req.Site)
	countryCode, countryName := resolveCountry(req.CountryCode, req.Country)

	pv := &model.PageView{
		Site:             site,
		CountryCode:      countryCode,
		Country:          countryName,
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

// GetStats returns aggregated analytics over the requested time period and site filter.
func (s *AnalyticsService) GetStats(ctx context.Context, period string, site string) (*model.AnalyticsStatsResponse, error) {
	return s.repo.GetAnalyticsStats(ctx, period, site)
}

// normalizeSite standardizes site identifiers across the ecosystem.
func normalizeSite(raw string) string {
	cleaned := strings.TrimSpace(strings.ToLower(raw))
	cleaned = strings.TrimPrefix(cleaned, "https://")
	cleaned = strings.TrimPrefix(cleaned, "http://")
	cleaned = strings.Split(cleaned, "/")[0]
	cleaned = strings.Split(cleaned, ":")[0]

	if strings.Contains(cleaned, "bin.irvanma.eu.org") {
		return "bin.irvanma.eu.org"
	}
	if strings.Contains(cleaned, "irvanma.eu.org") {
		return "irvanma.eu.org"
	}
	if cleaned == "" {
		return "irvanma.eu.org"
	}
	return cleaned
}

var countryNames = map[string]string{
	"US": "United States",
	"ID": "Indonesia",
	"SG": "Singapore",
	"JP": "Japan",
	"DE": "Germany",
	"GB": "United Kingdom",
	"CA": "Canada",
	"AU": "Australia",
	"FR": "France",
	"IN": "India",
	"NL": "Netherlands",
	"BR": "Brazil",
	"KR": "South Korea",
	"SE": "Sweden",
	"CH": "Switzerland",
	"IT": "Italy",
	"ES": "Spain",
	"RU": "Russia",
	"VN": "Vietnam",
	"MY": "Malaysia",
	"PH": "Philippines",
	"TH": "Thailand",
	"NZ": "New Zealand",
	"MX": "Mexico",
	"AR": "Argentina",
	"ZA": "South Africa",
	"PL": "Poland",
	"TR": "Turkey",
	"UA": "Ukraine",
	"NO": "Norway",
	"DK": "Denmark",
	"FI": "Finland",
	"IE": "Ireland",
	"AT": "Austria",
	"BE": "Belgium",
	"CZ": "Czech Republic",
	"PT": "Portugal",
	"RO": "Romania",
	"HU": "Hungary",
	"GR": "Greece",
	"IL": "Israel",
	"AE": "United Arab Emirates",
	"SA": "Saudi Arabia",
	"EG": "Egypt",
	"NG": "Nigeria",
	"KE": "Kenya",
	"CL": "Chile",
	"CO": "Colombia",
	"PE": "Peru",
	"PK": "Pakistan",
	"BD": "Bangladesh",
	"HK": "Hong Kong",
	"TW": "Taiwan",
}

// resolveCountry maps country code to standard country name.
func resolveCountry(code string, providedName string) (string, string) {
	cCode := strings.ToUpper(strings.TrimSpace(code))
	if cCode == "" || cCode == "XX" || cCode == "T1" {
		if strings.TrimSpace(providedName) != "" {
			return "", strings.TrimSpace(providedName)
		}
		return "", "Unknown"
	}
	if name, ok := countryNames[cCode]; ok {
		return cCode, name
	}
	if strings.TrimSpace(providedName) != "" {
		return cCode, strings.TrimSpace(providedName)
	}
	return cCode, cCode
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
