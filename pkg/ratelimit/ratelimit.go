// Package ratelimit provides a pluggable rate limiting interface and an
// in-memory sliding-window implementation for per-tenant / per-key throttling.
//
// The design is interface-based so callers can swap in a Redis-backed
// implementation later without changing any middleware code.
package ratelimit

import (
	"context"
	"sync"
	"time"
)

// Result holds the outcome of a single rate-limit check.
type Result struct {
	// Allowed reports whether the request is permitted.
	Allowed bool
	// Remaining is the number of requests left in the current window.
	Remaining int
	// Limit is the configured maximum number of requests per window.
	Limit int
	// ResetAt is the time when the window resets (i.e. oldest request drops off).
	ResetAt time.Time
}

// Limiter is the abstraction that all rate-limit backends must satisfy.
// Implementations must be safe for concurrent use.
type Limiter interface {
	// Allow checks whether a request identified by key is within the allowed
	// rate and records the attempt. key is typically:
	//   "rl:tenant:<tenantID>" or "rl:sa:<serviceAccountID>"
	Allow(ctx context.Context, key string) Result

	// Close releases any resources held by the limiter (e.g. background ticker).
	Close()
}

// NoopLimiter always allows every request. Useful for testing or when rate
// limiting is explicitly disabled.
type NoopLimiter struct{}

func (NoopLimiter) Allow(_ context.Context, _ string) Result {
	return Result{Allowed: true, Remaining: -1, Limit: -1, ResetAt: time.Time{}}
}
func (NoopLimiter) Close() {}

// ---------------------------------------------------------------------------
// In-memory sliding-window implementation
// ---------------------------------------------------------------------------

// InMemoryConfig holds tunables for the in-memory limiter.
type InMemoryConfig struct {
	// RequestsPerWindow is the maximum number of allowed requests within Window.
	RequestsPerWindow int
	// Window is the duration of the sliding window.
	Window time.Duration
}

// DefaultInMemoryConfig returns sane defaults: 300 requests per minute.
func DefaultInMemoryConfig() InMemoryConfig {
	return InMemoryConfig{
		RequestsPerWindow: 300,
		Window:            time.Minute,
	}
}

// inMemoryLimiter implements Limiter using a sliding-window approach backed
// by per-key timestamp queues stored in a sync.Map.
// It starts a background goroutine that periodically purges stale entries.
type inMemoryLimiter struct {
	cfg     InMemoryConfig
	mu      sync.Mutex
	buckets sync.Map // map[string]*bucket
	stopCh  chan struct{}
}

type bucket struct {
	mu        sync.Mutex
	timestamps []time.Time // sorted oldest-first
}

// NewInMemory creates a new in-memory sliding-window rate limiter.
// The returned limiter runs a background cleanup goroutine; call Close when done.
func NewInMemory(cfg InMemoryConfig) Limiter {
	if cfg.RequestsPerWindow <= 0 {
		cfg.RequestsPerWindow = 300
	}
	if cfg.Window <= 0 {
		cfg.Window = time.Minute
	}

	l := &inMemoryLimiter{
		cfg:    cfg,
		stopCh: make(chan struct{}),
	}
	go l.cleanupLoop()
	return l
}

func (l *inMemoryLimiter) Allow(_ context.Context, key string) Result {
	now := time.Now().UTC()
	windowStart := now.Add(-l.cfg.Window)

	bktRaw, _ := l.buckets.LoadOrStore(key, &bucket{})
	bkt := bktRaw.(*bucket)

	bkt.mu.Lock()
	defer bkt.mu.Unlock()

	// Evict timestamps older than the window.
	cutoff := 0
	for cutoff < len(bkt.timestamps) && bkt.timestamps[cutoff].Before(windowStart) {
		cutoff++
	}
	bkt.timestamps = bkt.timestamps[cutoff:]

	if len(bkt.timestamps) >= l.cfg.RequestsPerWindow {
		// Rate limited — oldest timestamp tells us when the window resets.
		resetAt := bkt.timestamps[0].Add(l.cfg.Window)
		return Result{
			Allowed:   false,
			Remaining: 0,
			Limit:     l.cfg.RequestsPerWindow,
			ResetAt:   resetAt,
		}
	}

	// Allow — record this request.
	bkt.timestamps = append(bkt.timestamps, now)
	remaining := l.cfg.RequestsPerWindow - len(bkt.timestamps)
	resetAt := now.Add(l.cfg.Window)
	if len(bkt.timestamps) > 0 {
		resetAt = bkt.timestamps[0].Add(l.cfg.Window)
	}
	return Result{
		Allowed:   true,
		Remaining: remaining,
		Limit:     l.cfg.RequestsPerWindow,
		ResetAt:   resetAt,
	}
}

func (l *inMemoryLimiter) Close() {
	select {
	case <-l.stopCh:
	default:
		close(l.stopCh)
	}
}

// cleanupLoop removes buckets that have had no activity for longer than one window.
func (l *inMemoryLimiter) cleanupLoop() {
	ticker := time.NewTicker(l.cfg.Window)
	defer ticker.Stop()

	for {
		select {
		case <-l.stopCh:
			return
		case <-ticker.C:
			l.purgeStale()
		}
	}
}

func (l *inMemoryLimiter) purgeStale() {
	cutoff := time.Now().UTC().Add(-l.cfg.Window)
	l.buckets.Range(func(k, v any) bool {
		bkt := v.(*bucket)
		bkt.mu.Lock()
		allStale := len(bkt.timestamps) == 0 ||
			(len(bkt.timestamps) > 0 && bkt.timestamps[len(bkt.timestamps)-1].Before(cutoff))
		bkt.mu.Unlock()

		if allStale {
			l.buckets.Delete(k)
		}
		return true
	})
}
