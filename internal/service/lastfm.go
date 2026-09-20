package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/irvanmalik48/realm-api/internal/model"
)

var (
	ErrMissingAPIKey = errors.New("missing LASTFM_API_KEY")
	ErrNotFound      = errors.New("resource not found")
	ErrUpstreamError = errors.New("failed to fetch LastFM data")
)

type UpstreamError struct {
	StatusCode int
	Message    string
}

func (e *UpstreamError) Error() string {
	return fmt.Sprintf("lastfm upstream error (%d): %s", e.StatusCode, e.Message)
}

type LastFMService interface {
	GetRecentTracks(ctx context.Context, username string, limit int) (*model.LastFMTrackResponseBody, error)
	GetUserInfo(ctx context.Context, username string) (*model.LastFMUserResponseBody, error)
}

type lastFMCacheItem struct {
	data      interface{}
	expiresAt time.Time
}

type lastFMService struct {
	apiKey     string
	apiSecret  string
	baseURL    string
	httpClient *http.Client
	cacheMu    sync.RWMutex
	cache      map[string]lastFMCacheItem
}

func NewLastFMService(apiKey, apiSecret string) LastFMService {
	return &lastFMService{
		apiKey:    apiKey,
		apiSecret: apiSecret,
		baseURL:   "https://ws.audioscrobbler.com/2.0/",
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		cache: make(map[string]lastFMCacheItem),
	}
}

func (s *lastFMService) GetRecentTracks(ctx context.Context, username string, limit int) (*model.LastFMTrackResponseBody, error) {
	if s.apiKey == "" {
		return nil, ErrMissingAPIKey
	}

	cacheKey := fmt.Sprintf("tracks:%s:%d", username, limit)
	s.cacheMu.RLock()
	item, ok := s.cache[cacheKey]
	if ok && time.Now().Before(item.expiresAt) {
		res := item.data.(*model.LastFMTrackResponseBody)
		s.cacheMu.RUnlock()
		return res, nil
	}
	s.cacheMu.RUnlock()

	endpoint, err := url.Parse(s.baseURL)
	if err != nil {
		return nil, err
	}

	q := endpoint.Query()
	q.Set("method", "user.getrecenttracks")
	q.Set("user", username)
	q.Set("api_key", s.apiKey)
	q.Set("format", "json")
	q.Set("limit", strconv.Itoa(limit))
	endpoint.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, &UpstreamError{
			StatusCode: resp.StatusCode,
			Message:    string(bodyBytes),
		}
	}

	var result model.LastFMTrackResponseBody
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	s.cacheMu.Lock()
	s.cache[cacheKey] = lastFMCacheItem{
		data:      &result,
		expiresAt: time.Now().Add(30 * time.Second),
	}
	s.cacheMu.Unlock()

	return &result, nil
}

func (s *lastFMService) GetUserInfo(ctx context.Context, username string) (*model.LastFMUserResponseBody, error) {
	if s.apiKey == "" {
		return nil, ErrMissingAPIKey
	}

	cacheKey := fmt.Sprintf("user:%s", username)
	s.cacheMu.RLock()
	item, ok := s.cache[cacheKey]
	if ok && time.Now().Before(item.expiresAt) {
		res := item.data.(*model.LastFMUserResponseBody)
		s.cacheMu.RUnlock()
		return res, nil
	}
	s.cacheMu.RUnlock()

	endpoint, err := url.Parse(s.baseURL)
	if err != nil {
		return nil, err
	}

	q := endpoint.Query()
	q.Set("method", "user.getinfo")
	q.Set("user", username)
	q.Set("api_key", s.apiKey)
	q.Set("format", "json")
	endpoint.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, &UpstreamError{
			StatusCode: resp.StatusCode,
			Message:    string(bodyBytes),
		}
	}

	var result model.LastFMUserResponseBody
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	s.cacheMu.Lock()
	s.cache[cacheKey] = lastFMCacheItem{
		data:      &result,
		expiresAt: time.Now().Add(10 * time.Minute),
	}
	s.cacheMu.Unlock()

	return &result, nil
}
