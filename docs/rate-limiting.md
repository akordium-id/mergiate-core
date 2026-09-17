# Rate Limiting

Documentation for the **Rate Limiting** engine in **Mergiate Core**.

---

## 1. Overview

Rate limiting restricts the number of HTTP requests that can be performed by a single tenant or service account within a designated time window. This feature is essential for:

- Protecting the platform against abuse (brute force, aggressive scraping, accidental infinite loops)
- Enabling tier-based usage policies (e.g., Starter, Pro, Enterprise)
- Safeguarding stability across shared multi-tenant resources

---

## 2. Configuration

Rate limiting is **disabled by default** and can be enabled via environment variables.

| Env Var | Default | Description |
|---|---|---|
| `RATE_LIMIT_ENABLED` | `false` | Set to `true` to activate rate limiting |
| `RATE_LIMIT_PER_MINUTE` | `300` | Maximum allowed requests per minute per bucket |
| `RATE_LIMIT_BURST` | `50` | Reserved for distributed token bucket algorithms |

Example `.env`:
```env
RATE_LIMIT_ENABLED=true
RATE_LIMIT_PER_MINUTE=300
```

---

## 3. Algorithm: Sliding Window Counter

The default implementation uses an **in-memory sliding window counter**:

- Every incoming request records a timestamp within the respective key bucket.
- Timestamps older than the sliding window (60 seconds) are automatically evicted.
- If the count of timestamps within the active window reaches or exceeds the configured limit, the request is rejected.
- A background ticker evicts stale, idle buckets periodically to prevent memory leaks.

> **Note**: The default in-memory implementation operates on a single process and does not synchronize state across multiple server instances. For distributed multi-instance clusters, implement the `ratelimit.Limiter` interface using a Redis backend.

---

## 4. Key Bucketing Strategy

The rate limiter evaluates client identities using the following key hierarchy:

| Client Context | Bucket Key |
|---|---|
| Request authenticated via **Service Account** (API Key) | `rl:sa:<serviceAccountID>` |
| Request authenticated via **User Session** (JWT Bearer) | `rl:tenant:<tenantID>` |
| Fallback (Unauthenticated / Public routes) | `rl:ip:<remoteAddr>` |

This ensures that an aggressive or misconfigured service account will not exhaust quota for other service accounts within the same tenant, nor impact neighboring tenants.

---

## 5. Response Headers

Every request processed by the rate limiting middleware includes RFC-compliant rate limit telemetry headers:

```http
X-RateLimit-Limit: 300
X-RateLimit-Remaining: 147
X-RateLimit-Reset: 1726540800
```

| Header | Description |
|---|---|
| `X-RateLimit-Limit` | Maximum request capacity allowed within the time window |
| `X-RateLimit-Remaining` | Remaining request quota available in the current window |
| `X-RateLimit-Reset` | Unix timestamp (in seconds) when the current window resets |

---

## 6. HTTP 429 Response

When a client exceeds their allocated threshold, the server immediately returns an HTTP 429 response with a `Retry-After` header:

```http
HTTP/1.1 429 Too Many Requests
Retry-After: 42
X-RateLimit-Limit: 300
X-RateLimit-Remaining: 0
X-RateLimit-Reset: 1726540842
Content-Type: application/json

{
  "error": {
    "code": "RATE_LIMIT_EXCEEDED",
    "message": "Rate limit exceeded. Try again in 42 seconds."
  }
}
```

Clients are expected to pause execution until the seconds specified in `Retry-After` have elapsed.

---

## 7. Middleware Pipeline Integration

The rate limiter is mounted directly downstream of authentication middleware:

```
Request → AuthRequired → RateLimit → TenantRequired → Controller Handler
```

This guarantees that `tenant_id` and `service_account_id` contexts have already been verified before the rate limit bucket key is computed.

---

## 8. Custom Backend Extension (e.g. Redis)

To replace the in-memory limiter with a distributed storage backend without modifying route handlers, provide an implementation conforming to the `ratelimit.Limiter` interface:

```go
// pkg/ratelimit/ratelimit.go
type Limiter interface {
    Allow(ctx context.Context, key string) Result
    Close()
}
```

And configure it during application assembly in `pkg/app/app.go`.
