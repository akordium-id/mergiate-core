# Getting Started with Mergiate Core

This guide covers setting up your local development environment, running background services with Docker, executing migrations, generating type-safe queries with `sqlc`, and launching the server.

---

## 1. Prerequisites

Make sure the following tools are installed on your machine:
- **Go**: Version 1.26 or higher
- **Docker & Docker Compose**: For running PostgreSQL 17 and Redis
- **golang-migrate**: For managing database schema migrations
- **sqlc**: (Optional) If you modify `.sql` files in `internal/core/repository/postgres/queries/`

---

## 2. Environment Configuration

Copy the example environment file:
```bash
cp .env.example .env
```

Default configuration variables:
```dotenv
APP_ENV=development
APP_PORT=8080
APP_NAME=mergiate-core
HTTP2_ENABLED=true

# Database (PostgreSQL 17)
DB_HOST=localhost
DB_PORT=5434
DB_USER=mergiate
DB_PASSWORD=mergiate_password
DB_NAME=mergiate_core
DB_SSLMODE=disable

# Connection Pool Tuning
DB_MAX_CONNS=15
DB_MIN_CONNS=3
DB_MAX_CONN_IDLE=15m
DB_MAX_CONN_LIFE=1h
DB_PGBOUNCER=false

# Redis 7
REDIS_HOST=localhost
REDIS_PORT=6380
```

> **Performance Notes**:
> - `HTTP2_ENABLED=true` enables HTTP/2 Cleartext (`h2c`) multiplexing, recommended for ConnectRPC and modern reverse proxies. Set to `false` if your proxy requires HTTP/1.1.
> - `DB_PGBOUNCER=true` disables named prepared statement caching, making the app fully compatible with PgBouncer transaction pooling, Supabase, Neon, or AWS RDS Proxy.
> - Default PostgreSQL port is `5434` and Redis port is `6380` to prevent collisions with existing host services.

---

## 3. Starting Local Infrastructure

Start PostgreSQL and Redis services in the background:
```bash
make docker-up
```

To stop containers:
```bash
make docker-down
```

---

## 4. Running Database Migrations

Apply all up migrations:
```bash
make migrate-up
```

To roll back the most recent migration:
```bash
make migrate-down
```

---

## 5. Compiling Queries with `sqlc`

Whenever you add or update SQL queries in `internal/core/repository/postgres/queries/`:
```bash
make sqlc
```
This generates Go code into `internal/core/repository/postgres/sqlc/`.

---

## 6. Running the Application

### Development Mode
```bash
make run
```

### Build Binary
```bash
make build
./bin/server
```

---

## 7. Verifying Endpoints

Check readiness:
```bash
curl -i http://localhost:8080/readyz
```

Create a new Tenant:
```bash
curl -i -X POST http://localhost:8080/api/v1/tenants \
  -H "Content-Type: application/json" \
  -d '{
    "code": "demo-corp",
    "name": "Demo Corporation",
    "settings": {
      "locale": "id_ID",
      "currency": "IDR"
    }
  }'
```

List Tenants:
```bash
curl -i http://localhost:8080/api/v1/tenants
```

---

## 8. Running Automated Tests

Run all unit and domain invariant tests:
```bash
make test
```
