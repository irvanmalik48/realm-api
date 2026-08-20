package auth

import (
	"sync"
	"time"
)

type tokenBucket struct {
	timestamps []time.Time
}

type TokenRateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*tokenBucket
	stopCh  chan struct{}
}

func NewTokenRateLimiter() *TokenRateLimiter {
	limiter := &TokenRateLimiter{
		buckets: make(map[string]*tokenBucket),
		stopCh:  make(chan struct{}),
	}

	go limiter.cleanupLoop(2 * time.Minute)

	return limiter
}

// Allow checks whether a request is allowed for a given identifier (e.g. token ID or IP)
// within a 1-minute sliding window.
func (l *TokenRateLimiter) Allow(id string, limitRPM int) (allowed bool, remaining int, resetEpoch int64) {
	if limitRPM <= 0 {
		return true, 999999, time.Now().Add(time.Minute).Unix()
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	windowStart := now.Add(-1 * time.Minute)

	bucket, exists := l.buckets[id]
	if !exists {
		bucket = &tokenBucket{timestamps: make([]time.Time, 0, limitRPM)}
		l.buckets[id] = bucket
	}

	// Filter timestamps within the current sliding minute
	validIdx := 0
	for i, t := range bucket.timestamps {
		if t.After(windowStart) {
			validIdx = i
			break
		}
		if i == len(bucket.timestamps)-1 {
			validIdx = len(bucket.timestamps)
		}
	}
	bucket.timestamps = bucket.timestamps[validIdx:]

	currentCount := len(bucket.timestamps)
	resetTime := now.Add(time.Minute)
	if currentCount > 0 {
		resetTime = bucket.timestamps[0].Add(time.Minute)
	}
	resetEpoch = resetTime.Unix()

	if currentCount >= limitRPM {
		return false, 0, resetEpoch
	}

	bucket.timestamps = append(bucket.timestamps, now)
	remaining = limitRPM - len(bucket.timestamps)
	if remaining < 0 {
		remaining = 0
	}

	return true, remaining, resetEpoch
}

func (l *TokenRateLimiter) Close() {
	close(l.stopCh)
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
	l.mu.Lock()
	defer l.mu.Unlock()

	threshold := time.Now().Add(-5 * time.Minute)
	for id, bucket := range l.buckets {
		if len(bucket.timestamps) == 0 || bucket.timestamps[len(bucket.timestamps)-1].Before(threshold) {
			delete(l.buckets, id)
		}
	}
}
