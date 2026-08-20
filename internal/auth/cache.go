package auth

import (
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/irvanmalik48/realm-api/internal/model"
)

type cachedToken struct {
	token     *model.APIToken
	expiresAt time.Time
}

type TokenCache struct {
	mu     sync.RWMutex
	items  map[string]cachedToken
	ttl    time.Duration
	stopCh chan struct{}
}

func NewTokenCache(ttl time.Duration) *TokenCache {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	tc := &TokenCache{
		items:  make(map[string]cachedToken),
		ttl:    ttl,
		stopCh: make(chan struct{}),
	}

	// Periodic cleanup every minute
	go tc.cleanupLoop(1 * time.Minute)

	return tc
}

func (tc *TokenCache) Get(tokenHash string) (*model.APIToken, bool) {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	item, exists := tc.items[tokenHash]
	if !exists {
		return nil, false
	}

	if time.Now().After(item.expiresAt) {
		return nil, false
	}

	return item.token, true
}

func (tc *TokenCache) Set(tokenHash string, token *model.APIToken) {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	tc.items[tokenHash] = cachedToken{
		token:     token,
		expiresAt: time.Now().Add(tc.ttl),
	}
}

func (tc *TokenCache) Invalidate(tokenHash string) {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	delete(tc.items, tokenHash)
}

func (tc *TokenCache) InvalidateByID(id uuid.UUID) {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	for k, v := range tc.items {
		if v.token != nil && v.token.ID == id {
			delete(tc.items, k)
		}
	}
}

func (tc *TokenCache) Close() {
	close(tc.stopCh)
}

func (tc *TokenCache) cleanupLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			tc.cleanup()
		case <-tc.stopCh:
			return
		}
	}
}

func (tc *TokenCache) cleanup() {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	now := time.Now()
	for k, v := range tc.items {
		if now.After(v.expiresAt) {
			delete(tc.items, k)
		}
	}
}
