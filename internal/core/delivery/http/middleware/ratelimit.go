package middleware

import (
	"fmt"
	"net/http"
	"time"

	"github.com/akordium-id/mergiate-core/pkg/ratelimit"
	"github.com/akordium-id/mergiate-core/pkg/response"
	"github.com/akordium-id/mergiate-core/pkg/sdk"
)

// RateLimit returns a middleware that throttles requests per tenant (or per service account).
// It reads the tenant ID from the authenticated context (injected by AuthRequired) and
// applies the limiter configured in pkg/ratelimit.
//
// Response headers on every allowed request:
//
//	X-RateLimit-Limit     — maximum requests per window
//	X-RateLimit-Remaining — remaining requests in the current window
//	X-RateLimit-Reset     — Unix timestamp (seconds) when the window resets
//
// On a rate-limited request the middleware returns 429 Too Many Requests and sets the
// Retry-After header in addition to the X-RateLimit-* headers.
func RateLimit(limiter ratelimit.Limiter) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := rateLimitKey(r)
			res := limiter.Allow(r.Context(), key)

			setRateLimitHeaders(w, res)

			if !res.Allowed {
				retryAfter := int(time.Until(res.ResetAt).Seconds())
				if retryAfter < 1 {
					retryAfter = 1
				}
				w.Header().Set("Retry-After", fmt.Sprintf("%d", retryAfter))
				response.Err(w, http.StatusTooManyRequests, "RATE_LIMIT_EXCEEDED",
					fmt.Sprintf("Rate limit exceeded. Try again in %d seconds.", retryAfter))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// rateLimitKey builds a stable limiter key from the request context.
// Priority: service account (API key actor) > tenant > IP fallback.
func rateLimitKey(r *http.Request) string {
	claims, ok := sdk.GetAuthClaims(r.Context())
	if ok && claims != nil {
		// Service account / API key actors get their own quota bucket so a single SA
		// cannot exhaust the entire tenant's allowance.
		if claims.ActorType == sdk.ActorTypeAPIKey && claims.ServiceAccountID != nil {
			return "rl:sa:" + claims.ServiceAccountID.String()
		}
		if claims.TenantID != sdk.NilID() {
			return "rl:tenant:" + claims.TenantID.String()
		}
	}
	// Fallback for unauthenticated contexts (should not reach here behind AuthRequired).
	return "rl:ip:" + r.RemoteAddr
}

func setRateLimitHeaders(w http.ResponseWriter, res ratelimit.Result) {
	if res.Limit < 0 {
		// NoopLimiter — skip headers to avoid noise.
		return
	}
	w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", res.Limit))
	w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", res.Remaining))
	if !res.ResetAt.IsZero() {
		w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", res.ResetAt.Unix()))
	}
}
