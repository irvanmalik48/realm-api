package model

import "time"

// TrackEventRequest represents the incoming pageview telemetry payload from client web pages.
type TrackEventRequest struct {
	Path             string `json:"path"`
	PostSlug         string `json:"post_slug,omitempty"`
	Referrer         string `json:"referrer,omitempty"`
	UserAgent        string `json:"user_agent,omitempty"`
	SessionID        string `json:"session_id,omitempty"`
	ScreenResolution string `json:"screen_resolution,omitempty"`
	IPHash           string `json:"ip_hash,omitempty"`
}

// PageView represents an individual pageview record in the database.
type PageView struct {
	ID               string    `json:"id"`
	Path             string    `json:"path"`
	PostSlug         *string   `json:"post_slug,omitempty"`
	Referrer         string    `json:"referrer,omitempty"`
	UserAgent        string    `json:"user_agent,omitempty"`
	Browser          string    `json:"browser,omitempty"`
	OS               string    `json:"os,omitempty"`
	DeviceType       string    `json:"device_type,omitempty"`
	SessionID        string    `json:"session_id,omitempty"`
	ScreenResolution string    `json:"screen_resolution,omitempty"`
	IPHash           string    `json:"ip_hash,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
}

// TrendPoint represents pageviews aggregated by date or hour.
type TrendPoint struct {
	Date   string `json:"date"`
	Views  int64  `json:"views"`
	Unique int64  `json:"unique"`
}

// PageStat represents pageview metrics for a specific URL path.
type PageStat struct {
	Path   string `json:"path"`
	Views  int64  `json:"views"`
	Unique int64  `json:"unique"`
}

// PostStat represents analytics metrics for a specific blog post.
type PostStat struct {
	Slug   string `json:"slug"`
	Title  string `json:"title,omitempty"`
	Views  int64  `json:"views"`
	Unique int64  `json:"unique"`
}

// ReferrerStat represents traffic source breakdown.
type ReferrerStat struct {
	Referrer string `json:"referrer"`
	Count    int64  `json:"count"`
}

// DeviceStat represents device category breakdown (Desktop, Mobile, Tablet).
type DeviceStat struct {
	Device string `json:"device"`
	Count  int64  `json:"count"`
}

// BrowserStat represents browser distribution.
type BrowserStat struct {
	Browser string `json:"browser"`
	Count   int64  `json:"count"`
}

// RecentPageView represents an individual recent visit for live feeds.
type RecentPageView struct {
	ID        string    `json:"id"`
	Path      string    `json:"path"`
	PostSlug  *string   `json:"post_slug,omitempty"`
	Referrer  string    `json:"referrer,omitempty"`
	Browser   string    `json:"browser,omitempty"`
	OS        string    `json:"os,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// AnalyticsStatsResponse contains aggregated analytics metrics for dashboards.
type AnalyticsStatsResponse struct {
	TotalViews     int64            `json:"total_views"`
	UniqueVisitors int64            `json:"unique_visitors"`
	ViewsToday     int64            `json:"views_today"`
	ViewsTrend     []TrendPoint     `json:"views_trend"`
	TopPages       []PageStat       `json:"top_pages"`
	TopPosts       []PostStat       `json:"top_posts"`
	TopReferrers   []ReferrerStat   `json:"top_referrers"`
	DeviceStats    []DeviceStat     `json:"device_stats"`
	BrowserStats   []BrowserStat    `json:"browser_stats"`
	RecentViews    []RecentPageView `json:"recent_views"`
}
