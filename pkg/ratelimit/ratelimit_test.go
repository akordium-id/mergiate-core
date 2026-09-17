package ratelimit_test

import (
	"context"
	"testing"
	"time"

	"github.com/akordium-id/mergiate-core/pkg/ratelimit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInMemory_AllowsUnderLimit(t *testing.T) {
	l := ratelimit.NewInMemory(ratelimit.InMemoryConfig{
		RequestsPerWindow: 5,
		Window:            time.Minute,
	})
	defer l.Close()

	ctx := context.Background()
	for i := range 5 {
		res := l.Allow(ctx, "tenant:abc")
		require.True(t, res.Allowed, "request %d should be allowed", i+1)
		assert.Equal(t, 5, res.Limit)
	}
}

func TestInMemory_BlocksOverLimit(t *testing.T) {
	l := ratelimit.NewInMemory(ratelimit.InMemoryConfig{
		RequestsPerWindow: 3,
		Window:            time.Minute,
	})
	defer l.Close()

	ctx := context.Background()
	for range 3 {
		l.Allow(ctx, "tenant:xyz")
	}

	res := l.Allow(ctx, "tenant:xyz")
	assert.False(t, res.Allowed, "4th request should be blocked")
	assert.Equal(t, 0, res.Remaining)
	assert.True(t, res.ResetAt.After(time.Now()))
}

func TestInMemory_IsolatesKeys(t *testing.T) {
	l := ratelimit.NewInMemory(ratelimit.InMemoryConfig{
		RequestsPerWindow: 2,
		Window:            time.Minute,
	})
	defer l.Close()

	ctx := context.Background()
	// Exhaust key A
	l.Allow(ctx, "tenant:a")
	l.Allow(ctx, "tenant:a")
	resA := l.Allow(ctx, "tenant:a")
	assert.False(t, resA.Allowed, "tenant:a should be rate-limited")

	// Key B should still be fine
	resB := l.Allow(ctx, "tenant:b")
	assert.True(t, resB.Allowed, "tenant:b should not be affected by tenant:a")
}

func TestInMemory_RemainingDecrementsCorrectly(t *testing.T) {
	l := ratelimit.NewInMemory(ratelimit.InMemoryConfig{
		RequestsPerWindow: 10,
		Window:            time.Minute,
	})
	defer l.Close()

	ctx := context.Background()
	for i := 9; i >= 0; i-- {
		res := l.Allow(ctx, "tenant:rem")
		require.True(t, res.Allowed)
		assert.Equal(t, i, res.Remaining)
	}
}

func TestNoopLimiter_AlwaysAllows(t *testing.T) {
	l := ratelimit.NoopLimiter{}
	ctx := context.Background()
	for range 1000 {
		res := l.Allow(ctx, "any-key")
		assert.True(t, res.Allowed)
	}
}
