# Rate Limiting

Dokumentasi fitur **Rate Limiting** di platform **Mergiate Core**.

---

## 1. Overview

Rate limiting membatasi jumlah HTTP request yang bisa dilakukan oleh satu tenant atau satu service account dalam satu periode waktu tertentu. Fitur ini penting untuk:

- Melindungi platform dari abuse (brute force, scraping)
- Memungkinkan penagihan berbasis tier (Starter/Pro/Enterprise)
- Menjaga stabilitas server multi-tenant

---

## 2. Konfigurasi

Rate limiting **dinonaktifkan secara default** dan harus diaktifkan via environment variable.

| Env Var | Default | Deskripsi |
|---|---|---|
| `RATE_LIMIT_ENABLED` | `false` | `true` untuk mengaktifkan rate limiting |
| `RATE_LIMIT_PER_MINUTE` | `300` | Jumlah request maksimum per menit per bucket |
| `RATE_LIMIT_BURST` | `50` | Reserved untuk Redis token bucket (future) |

Contoh `.env`:
```env
RATE_LIMIT_ENABLED=true
RATE_LIMIT_PER_MINUTE=300
```

---

## 3. Algoritma: Sliding Window Counter

Implementasi default menggunakan **in-memory sliding window** (bukan token bucket):

- Setiap request dicatat timestamp-nya per key bucket.
- Saat request masuk, timestamp yang lebih lama dari 1 menit di-evict.
- Jika jumlah timestamp dalam window ≥ limit → request ditolak.
- Background goroutine membersihkan bucket stale setiap 1 menit.

> **Catatan**: Implementasi in-memory tidak persisten lintas restart server dan tidak shared antar instance (single-process only). Untuk multi-instance deployment, ganti implementasi via interface `ratelimit.Limiter` dengan backend Redis.

---

## 4. Bucketing: Siapa yang di-throttle?

Rate limiter menggunakan key hierarchy berikut (prioritas dari atas):

| Kondisi | Key Bucket |
|---|---|
| Request dari **Service Account** (API Key) | `rl:sa:<serviceAccountID>` |
| Request dari **User** (JWT Bearer) | `rl:tenant:<tenantID>` |
| Fallback (tidak terauth — tidak seharusnya terjadi) | `rl:ip:<remoteAddr>` |

Artinya: satu Service Account yang agresif **tidak akan menghabiskan quota** tenant lain, dan service account satu tidak mempengaruhi service account lain dalam tenant yang sama.

---

## 5. Response Headers

Setiap request yang melewati rate limiter — baik allowed maupun blocked — mendapatkan headers informatif:

```http
X-RateLimit-Limit: 300
X-RateLimit-Remaining: 147
X-RateLimit-Reset: 1726540800
```

| Header | Nilai |
|---|---|
| `X-RateLimit-Limit` | Batas maksimum request per window |
| `X-RateLimit-Remaining` | Sisa request dalam window saat ini |
| `X-RateLimit-Reset` | Unix timestamp (detik) saat window reset |

---

## 6. Response 429

Jika rate limit terlampaui, server merespons dengan:

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

Client harus menghormati header `Retry-After` sebelum mencoba ulang.

---

## 7. Posisi di Middleware Stack

Rate limiter dipasang **setelah `AuthRequired`** di semua route `/api/v1` yang ter-protect:

```
Request → AuthRequired → RateLimit → TenantRequired → Handler
```

Urutan ini penting karena rate limiter membutuhkan `tenant_id` / `service_account_id` dari context yang di-inject oleh `AuthRequired`.

---

## 8. Menambahkan Backend Redis (Future)

Untuk upgrade ke Redis tanpa mengubah middleware, implementasikan interface `ratelimit.Limiter`:

```go
// pkg/ratelimit/ratelimit.go
type Limiter interface {
    Allow(ctx context.Context, key string) Result
    Close()
}
```

Contoh konfigurasi di `pkg/app/app.go`:
```go
// Ganti:
limiter = ratelimit.NewInMemory(cfg)

// Dengan implementasi Redis custom:
limiter = myredis.NewRedisLimiter(redisClient, cfg)
```

---

## 9. Tier-based Rate Limiting (Roadmap)

Untuk paid cloud service dengan tier berbeda (Starter/Pro/Enterprise), bisa extend dengan menyimpan limit per-tenant di database:

```
tenants.rate_limit_per_minute INT DEFAULT 300
```

Lalu pass per-tenant limit ke `Limiter.Allow()` (perlu extend interface). Saat ini semua tenant menggunakan limit global dari env.
