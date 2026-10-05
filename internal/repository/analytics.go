package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/irvanmalik48/realm-api/internal/database"
	"github.com/irvanmalik48/realm-api/internal/model"
)

// AnalyticsRepository handles persistence and aggregations of web pageview events.
type AnalyticsRepository struct {
	db *database.DB
}

// NewAnalyticsRepository constructs an AnalyticsRepository.
func NewAnalyticsRepository(db *database.DB) *AnalyticsRepository {
	return &AnalyticsRepository{db: db}
}

// RecordPageView inserts a new pageview record into PostgreSQL.
func (r *AnalyticsRepository) RecordPageView(ctx context.Context, pv *model.PageView) error {
	query := `
		INSERT INTO page_views (
			path, post_slug, referrer, user_agent, browser, os,
			device_type, session_id, screen_resolution, ip_hash
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	_, err := r.db.Pool.Exec(ctx, query,
		pv.Path,
		pv.PostSlug,
		pv.Referrer,
		pv.UserAgent,
		pv.Browser,
		pv.OS,
		pv.DeviceType,
		pv.SessionID,
		pv.ScreenResolution,
		pv.IPHash,
	)
	return err
}

// GetAnalyticsStats computes aggregate metrics over the specified period ("24h", "7d", "30d", "all").
func (r *AnalyticsRepository) GetAnalyticsStats(ctx context.Context, period string) (*model.AnalyticsStatsResponse, error) {
	var since time.Time
	var timeBucket string
	var timeFormat string

	now := time.Now().UTC()
	switch strings.ToLower(period) {
	case "24h":
		since = now.Add(-24 * time.Hour)
		timeBucket = "YYYY-MM-DD HH24:00"
		timeFormat = "15:04"
	case "7d":
		since = now.Add(-7 * 24 * time.Hour)
		timeBucket = "YYYY-MM-DD"
		timeFormat = "Jan 02"
	case "all":
		since = time.Time{} // beginning of time
		timeBucket = "YYYY-MM-DD"
		timeFormat = "Jan 02"
	case "30d":
		fallthrough
	default:
		since = now.Add(-30 * 24 * time.Hour)
		timeBucket = "YYYY-MM-DD"
		timeFormat = "Jan 02"
	}

	res := &model.AnalyticsStatsResponse{
		ViewsTrend:   make([]model.TrendPoint, 0),
		TopPages:     make([]model.PageStat, 0),
		TopPosts:     make([]model.PostStat, 0),
		TopReferrers: make([]model.ReferrerStat, 0),
		DeviceStats:  make([]model.DeviceStat, 0),
		BrowserStats: make([]model.BrowserStat, 0),
		RecentViews:  make([]model.RecentPageView, 0),
	}

	whereClause := ""
	args := []any{}
	if !since.IsZero() {
		whereClause = "WHERE created_at >= $1"
		args = append(args, since)
	}

	// 1. Total views & Unique visitors
	countQuery := fmt.Sprintf(`
		SELECT 
			COUNT(*),
			COUNT(DISTINCT COALESCE(NULLIF(session_id, ''), NULLIF(ip_hash, ''), id::text))
		FROM page_views
		%s
	`, whereClause)

	if err := r.db.Pool.QueryRow(ctx, countQuery, args...).Scan(&res.TotalViews, &res.UniqueVisitors); err != nil {
		return nil, fmt.Errorf("failed to count total views: %w", err)
	}

	// 2. Views today
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	_ = r.db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM page_views WHERE created_at >= $1`, todayStart).Scan(&res.ViewsToday)

	// 3. Views Trend
	trendQuery := fmt.Sprintf(`
		SELECT 
			TO_CHAR(created_at, '%s') as bucket,
			COUNT(*) as views,
			COUNT(DISTINCT COALESCE(NULLIF(session_id, ''), NULLIF(ip_hash, ''), id::text)) as unq
		FROM page_views
		%s
		GROUP BY bucket
		ORDER BY bucket ASC
	`, timeBucket, whereClause)

	trendRows, err := r.db.Pool.Query(ctx, trendQuery, args...)
	if err == nil {
		defer trendRows.Close()
		for trendRows.Next() {
			var pt model.TrendPoint
			if err := trendRows.Scan(&pt.Date, &pt.Views, &pt.Unique); err == nil {
				res.ViewsTrend = append(res.ViewsTrend, pt)
			}
		}
	}

	// 4. Top Pages
	pagesQuery := fmt.Sprintf(`
		SELECT 
			path,
			COUNT(*) as views,
			COUNT(DISTINCT COALESCE(NULLIF(session_id, ''), NULLIF(ip_hash, ''), id::text)) as unq
		FROM page_views
		%s
		GROUP BY path
		ORDER BY views DESC
		LIMIT 10
	`, whereClause)

	pageRows, err := r.db.Pool.Query(ctx, pagesQuery, args...)
	if err == nil {
		defer pageRows.Close()
		for pageRows.Next() {
			var p model.PageStat
			if err := pageRows.Scan(&p.Path, &p.Views, &p.Unique); err == nil {
				res.TopPages = append(res.TopPages, p)
			}
		}
	}

	// 5. Top Posts
	postsQuery := fmt.Sprintf(`
		SELECT 
			pv.post_slug,
			COALESCE(p.title, pv.post_slug) as title,
			COUNT(*) as views,
			COUNT(DISTINCT COALESCE(NULLIF(pv.session_id, ''), NULLIF(pv.ip_hash, ''), pv.id::text)) as unq
		FROM page_views pv
		LEFT JOIN posts p ON p.slug = pv.post_slug
		WHERE pv.post_slug IS NOT NULL AND pv.post_slug != ''
		%s
		GROUP BY pv.post_slug, p.title
		ORDER BY views DESC
		LIMIT 10
	`, func() string {
		if !since.IsZero() {
			return "AND pv.created_at >= $1"
		}
		return ""
	}())

	postRows, err := r.db.Pool.Query(ctx, postsQuery, args...)
	if err == nil {
		defer postRows.Close()
		for postRows.Next() {
			var ps model.PostStat
			if err := postRows.Scan(&ps.Slug, &ps.Title, &ps.Views, &ps.Unique); err == nil {
				res.TopPosts = append(res.TopPosts, ps)
			}
		}
	}

	// 6. Top Referrers
	refQuery := fmt.Sprintf(`
		SELECT 
			COALESCE(NULLIF(referrer, ''), 'Direct / None') as ref,
			COUNT(*) as count
		FROM page_views
		%s
		GROUP BY ref
		ORDER BY count DESC
		LIMIT 8
	`, whereClause)

	refRows, err := r.db.Pool.Query(ctx, refQuery, args...)
	if err == nil {
		defer refRows.Close()
		for refRows.Next() {
			var rf model.ReferrerStat
			if err := refRows.Scan(&rf.Referrer, &rf.Count); err == nil {
				res.TopReferrers = append(res.TopReferrers, rf)
			}
		}
	}

	// 7. Device Stats
	devQuery := fmt.Sprintf(`
		SELECT 
			COALESCE(NULLIF(device_type, ''), 'Desktop') as dev,
			COUNT(*) as count
		FROM page_views
		%s
		GROUP BY dev
		ORDER BY count DESC
	`, whereClause)

	devRows, err := r.db.Pool.Query(ctx, devQuery, args...)
	if err == nil {
		defer devRows.Close()
		for devRows.Next() {
			var ds model.DeviceStat
			if err := devRows.Scan(&ds.Device, &ds.Count); err == nil {
				res.DeviceStats = append(res.DeviceStats, ds)
			}
		}
	}

	// 8. Browser Stats
	browserQuery := fmt.Sprintf(`
		SELECT 
			COALESCE(NULLIF(browser, ''), 'Other') as brw,
			COUNT(*) as count
		FROM page_views
		%s
		GROUP BY brw
		ORDER BY count DESC
		LIMIT 6
	`, whereClause)

	browserRows, err := r.db.Pool.Query(ctx, browserQuery, args...)
	if err == nil {
		defer browserRows.Close()
		for browserRows.Next() {
			var bs model.BrowserStat
			if err := browserRows.Scan(&bs.Browser, &bs.Count); err == nil {
				res.BrowserStats = append(res.BrowserStats, bs)
			}
		}
	}

	// 9. Recent Views (last 15)
	recentQuery := `
		SELECT 
			id, path, post_slug,
			COALESCE(NULLIF(referrer, ''), 'Direct') as referrer,
			COALESCE(NULLIF(browser, ''), 'Unknown') as browser,
			COALESCE(NULLIF(os, ''), 'Unknown') as os,
			created_at
		FROM page_views
		ORDER BY created_at DESC
		LIMIT 15
	`
	recentRows, err := r.db.Pool.Query(ctx, recentQuery)
	if err == nil {
		defer recentRows.Close()
		for recentRows.Next() {
			var rv model.RecentPageView
			if err := recentRows.Scan(&rv.ID, &rv.Path, &rv.PostSlug, &rv.Referrer, &rv.Browser, &rv.OS, &rv.CreatedAt); err == nil {
				res.RecentViews = append(res.RecentViews, rv)
			}
		}
	}

	_ = timeFormat // suppress unused warning if necessary
	return res, nil
}
