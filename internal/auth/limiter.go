package auth

import (
	"hash/fnv"
	"math"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

const (
	numShards        = 32
	maxKeysPerShard  = 2048 // Total capacity across shards: 65,536 keys
	idleEvictTimeout = 3 * time.Minute
)

type limiterEntry struct {
	limiter  *rate.Limiter
	limitRPM int
	lastSeen time.Time
}

type limiterShard struct {
	mu      sync.RWMutex
	entries map[string]*limiterEntry
}

type TokenRateLimiter struct {
	shards [numShards]*limiterShard
	stopCh chan struct{}
}

func NewTokenRateLimiter() *TokenRateLimiter {
	trl := &TokenRateLimiter{
		stopCh: make(chan struct{}),
	}
	for i := 0; i < numShards; i++ {
		trl.shards[i] = &limiterShard{
			entries: make(map[string]*limiterEntry),
		}
	}

	go trl.cleanupLoop(1 * time.Minute)

	return trl
}

func (l *TokenRateLimiter) getShard(key string) *limiterShard {
	h := fnv.New32a()
	_, _ = h.Write([]byte(key))
	idx := h.Sum32() % numShards
	return l.shards[idx]
}

// Allow checks whether a request is allowed for a given identifier within a 1-minute window
func (l *TokenRateLimiter) Allow(id string, limitRPM int) (allowed bool, remaining int, resetEpoch int64) {
	if limitRPM <= 0 {
		return true, 999999, time.Now().Add(time.Minute).Unix()
	}

	now := time.Now()
	shard := l.getShard(id)

	shard.mu.RLock()
	entry, exists := shard.entries[id]
	if exists && entry.limitRPM == limitRPM {
		tokens := int(math.Floor(entry.limiter.Tokens()))
		allowed = entry.limiter.Allow()
		entry.lastSeen = now
		shard.mu.RUnlock()

		if allowed {
			rem := tokens - 1
			if rem < 0 {
				rem = 0
			}
			return true, rem, now.Add(time.Minute).Unix()
		}
		return false, 0, now.Add(time.Minute).Unix()
	}
	shard.mu.RUnlock()

	shard.mu.Lock()
	defer shard.mu.Unlock()

	// Recheck after acquiring write lock
	entry, exists = shard.entries[id]
	if !exists || entry.limitRPM != limitRPM {
		// Enforce capacity bound per shard to prevent unbounded memory growth
		if len(shard.entries) >= maxKeysPerShard {
			threshold := now.Add(-idleEvictTimeout)
			for k, v := range shard.entries {
				if v.lastSeen.Before(threshold) {
					delete(shard.entries, k)
				}
			}
			// If still full, evict the current oldest entry
			if len(shard.entries) >= maxKeysPerShard {
				for k := range shard.entries {
					delete(shard.entries, k)
					break
				}
			}
		}

		limitPerSec := rate.Limit(float64(limitRPM) / 60.0)
		entry = &limiterEntry{
			limiter:  rate.NewLimiter(limitPerSec, limitRPM),
			limitRPM: limitRPM,
			lastSeen: now,
		}
		shard.entries[id] = entry
	}

	entry.lastSeen = now
	tokens := int(math.Floor(entry.limiter.Tokens()))
	allowed = entry.limiter.Allow()

	if allowed {
		rem := tokens - 1
		if rem < 0 {
			rem = 0
		}
		return true, rem, now.Add(time.Minute).Unix()
	}
	return false, 0, now.Add(time.Minute).Unix()
}

func (l *TokenRateLimiter) Close() {
	select {
	case <-l.stopCh:
		// Already closed
	default:
		close(l.stopCh)
	}
}

func (l *TokenRateLimiter) cleanupLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			l.cleanup()
		case <-l.stopCh:
			return
		}
	}
}

func (l *TokenRateLimiter) cleanup() {
	now := time.Now()
	threshold := now.Add(-idleEvictTimeout)

	for _, shard := range l.shards {
		shard.mu.Lock()
		for id, entry := range shard.entries {
			if entry.lastSeen.Before(threshold) {
				delete(shard.entries, id)
			}
		}
		shard.mu.Unlock()
	}
}
