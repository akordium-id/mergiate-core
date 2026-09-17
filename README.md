# Mergiate Core

> **Connect. Operate. Grow.**  
> An open, modular ERP foundation and Business Operating Platform built with Go and PostgreSQL.

[![Go Version](https://img.shields.io/badge/go-1.26-blue.svg)](https://golang.org)
[![PostgreSQL](https://img.shields.io/badge/postgresql-17-blue.svg)](https://www.postgresql.org)
[![License](https://img.shields.io/badge/license-Apache--2.0-green.svg)](LICENSE)

Mergiate Core provides the essential primitives, domain boundaries, and multi-tenant foundation for modern enterprise resource planning systems. Rather than building a rigid monolith, Mergiate separates universal business primitives from domain-specific modules (Sales, Inventory, Accounting, HR, etc.).

---

## Architecture Principles

1. **Core Primitives vs. Module-Owned**: The core never imports or depends on specific business modules. Modules import the core contracts and primitives.
2. **Strict Aggregate Boundaries**: Cross-aggregate references use IDs (e.g., `PartyID`, `ProductID`), never direct nested object ownership.
3. **Immutable Value Objects**: Financial calculations (`Money`) and measurements (`Quantity`, `Unit`) are guarded by strict invariant rules (currency and unit mismatch protection).
4. **Time-Ordered UUIDv7**: Internal primary keys use UUIDv7 for distributed uniqueness and time-based index locality.
5. **Multi-Tenancy Guardrails**: Multi-tenant isolation is enforced at the application context layer and supported by database row-level boundaries.

---

## Tech Stack

- **Language**: Go 1.26
- **HTTP Router**: [Chi v5](https://github.com/go-chi/chi) (`net/http` standard library compliant)
- **Database**: PostgreSQL 17
- **Database Driver**: [pgx/v5](https://github.com/jackc/pgx) connection pool (`pgxpool`)
- **Query Compiler**: [sqlc](https://sqlc.dev) (compile-time verified, zero-reflection type-safe SQL)
- **Cache & Messaging**: Redis 7
- **Logging**: Structured JSON with Go standard library `log/slog`

---

## Directory Structure

```
mergiate-core/
├── cmd/
│   └── server/                         # Main HTTP application entrypoint
├── internal/
│   └── core/
│       ├── domain/
│       │   ├── shared/                 # Core Value Objects (ID, Money, Quantity, Tenant)
│       │   └── repository/             # Domain repository interfaces
│       ├── usecase/
│       │   └── tenant/                 # Application services & business logic
│       ├── repository/
│       │   └── postgres/               # PostgreSQL implementations (sqlc queries & models)
│       └── delivery/
│           └── http/
│               ├── middleware/         # Logging, Tenant extraction & recovery
│               └── v1/                 # Versioned REST HTTP handlers
├── pkg/
│   ├── config/                         # Environment configuration parser
│   ├── database/                       # PostgreSQL pgxpool manager
│   └── response/                       # Standardized JSON response envelopes
├── migrations/                         # SQL migration files
├── docs/                               # In-depth architectural & domain documentation
├── docker-compose.yml                  # Local development services (PostgreSQL 17, Redis 7)
├── sqlc.yaml                           # sqlc code generation configuration
└── Makefile                            # Development & automation shortcuts
```

---

## Quick Start

### 1. Prerequisites
- [Go 1.26+](https://golang.org/dl/)
- [Docker](https://docs.docker.com/get-docker/) & Docker Compose
- [golang-migrate](https://github.com/golang-migrate/migrate) CLI (optional for manual migrations)

### 2. Run Local Infrastructure
```bash
# Start PostgreSQL 17 and Redis 7 in background
make docker-up
```

### 3. Run Database Migrations
```bash
make migrate-up
```

### 4. Run the Server
```bash
make run
```
The server will start listening on `http://localhost:8080`.

### 5. Verify Service Health
```bash
# Check service liveness
curl http://localhost:8080/healthz

# Check database readiness
curl http://localhost:8080/readyz
```

---

## Running Tests

Execute all unit and domain invariant tests:
```bash
make test
```

---

## Documentation

Comprehensive guides and architectural specifications are available in the [`docs/`](docs/) directory:
- [Architecture Overview](docs/architecture.md)
- [Domain Primitives & Value Objects](docs/domain-primitives.md)
- [Organization & Party Model](docs/organization-and-party.md)
- [Product & Document Engine](docs/product-and-document.md)
- [Audit Trail & Outbox Engine](docs/audit-and-event-outbox.md)
- [Identity & Access Management (RBAC)](docs/identity-and-access.md)
- [Custom Field & Extension Engine](docs/custom-fields.md)
- [Document Numbering & Sequence Engine](docs/document-sequences.md)
- [File & Attachment Engine](docs/file-and-attachment.md)
- [Communication & Activity Timeline](docs/communication-and-notifications.md)
- [Module & Plugin SPI Engine](docs/module-plugin-system.md)
- [M2M API Keys & Service Accounts](docs/service-accounts-and-api-keys.md)
- [Rate Limiting](docs/rate-limiting.md)
- [Webhook Engine](docs/webhooks.md)
- [M2M RPC Protocol (gRPC & ConnectRPC)](docs/rpc-protocol.md)
- [Getting Started & Local Development](docs/getting-started.md)

---

## License

Apache License 2.0. See [LICENSE](LICENSE) for details.
