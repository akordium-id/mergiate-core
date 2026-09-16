# Mergiate Core: Architecture Guide

## 1. Overview & Vision

Mergiate Core is designed as an open-core, modular Business Operating Platform. Unlike monolithic ERP systems that tightly couple CRM, inventory, sales, and general ledger into an unmaintainable codebase, Mergiate divides the system into **Core Primitives** and **Module-Owned Entities**.

```
                   AKORDIUM ERP / MERGIATE
                             │
            ┌────────────────┴────────────────┐
            │                                 │
       CORE PRIMITIVES                  MODULE-OWNED
            │                                 │
            ├─ Tenant                         ├─ Sales Order
            ├─ Organization                   ├─ Invoice
            ├─ Party                          ├─ Purchase Order
            ├─ Product                        ├─ Stock Movement
            ├─ Document                       ├─ Payment
            ├─ Transaction                    ├─ Journal Entry
            ├─ Workflow                       ├─ Employee
            ├─ Event                          ├─ Production Order
            ├─ Money (Value Object)           └─ ...
            ├─ Quantity (Value Object)
            ├─ Unit (UoM)
            ├─ Address
            ├─ Contact
            └─ Custom Field
```

---

## 2. The 7 Architectural Invariants

To keep the core lean, reusable, and resistant to complexity rot, Mergiate adheres to 7 strict rules:

1. **Rule 1 — Core Never Imports Modules**:
   - `Core` $\to$ `Sales` ❌
   - `Sales` $\to$ `Core` ✅
   - Core provides contracts, types, and primitives. Modules depend on Core, never the reverse.
2. **Rule 2 — Module Entities Never Masquerade as Core Primitives**:
   - Just because multiple modules need an `Invoice` does not mean `Invoice` belongs in Core. Invoices belong to Sales/Accounting modules.
3. **Rule 3 — Reference by ID, Not Object Ownership**:
   - `SalesOrder` references `customer_party_id` and `product_id`. It does not embed `Party` or `Product` aggregates within its database table or aggregate root.
4. **Rule 4 — Money and Quantity are Immutable Value Objects**:
   - They have no independent database lifecycle or mutable state. Calculations enforce currency and unit compatibility at compile-time/runtime.
5. **Rule 5 — Events are Asynchronous Communication Boundaries**:
   - Modules do not invoke internal methods of other modules directly; they emit and consume domain events via the event bus or transactional outbox.
6. **Rule 6 — Custom Fields are the Extension Mechanism**:
   - Ad-hoc business attributes are supported via typed `CustomFieldDefinition` and `CustomFieldValue`, preventing arbitrary schema pollution in core tables.
7. **Rule 7 — Multi-Tenancy is an Absolute Security Boundary**:
   - Every business aggregate belongs to a `Tenant`. Access is enforced across application context and database queries.

---

## 3. Clean Architecture Layering

Mergiate strictly implements Clean Architecture in Go:

```
HTTP Request
     ↓
[Delivery Layer: Chi Handlers & Middleware]
     ↓ (Commands / DTOs)
[Usecase Layer: Application Services]
     ↓ (Entity manipulation & invariants)
[Domain Layer: Pure Entities, Value Objects, Repository Interfaces]
     ↑ (Implements Repository Interfaces)
[Repository Layer: PostgreSQL with sqlc & pgx/v5]
```

### Dependency Direction
- **`internal/core/domain`**: Zero external dependencies (except standard library and UUID). Contains entities, value objects, and repository interfaces.
- **`internal/core/usecase`**: Coordinates business workflows and invokes domain methods and repository interfaces.
- **`internal/core/repository/postgres`**: Implements domain interfaces using `sqlc` compiled queries and `pgxpool.Pool`.
- **`internal/core/delivery/http`**: Translates HTTP requests into usecase commands, formats JSON envelopes, and handles HTTP-level middleware.

---

## 4. ID Strategy: UUIDv7

Mergiate Core standardizes on **UUIDv7** for all internal primary keys:

- **Format**: `0199xxxx-xxxx-7xxx-xxxx-xxxxxxxxxxxx`
- **Time-Ordered**: 48-bit UNIX timestamp prefix ensures sequential database index insertions, eliminating B-Tree index fragmentation common with random UUIDv4.
- **Distributed Safety**: Keys can be generated on client, worker, or API nodes without database sequence locks.
- **Human-Readable Separation**: Internal IDs are UUIDv7; user-facing document numbers (`INV-2026-0001`, `SO-2026-0001`) are business-level identifiers managed by document sequence generators.

---

## 5. Multi-Tenancy Architecture

A `Tenant` represents the top-level administrative boundary:

```
Tenant
  ├── Organization (Company / Holding)
  │     ├── Branch
  │     └── Department
  ├── Users & Roles
  └── Business Entities (Party, Product, Document)
```

### Context Propagation
The HTTP layer extracts the tenant identity via the `X-Tenant-ID` header using [`TenantRequired()`](file:///home/orin/code/archive/mergiate-core/internal/core/delivery/http/middleware/tenant.go) middleware and injects it into `context.Context`:

```go
ctx := shared.WithTenantID(r.Context(), tenantID)
```

Usecases and repositories retrieve this context using `shared.RequireTenantID(ctx)` to guarantee that database queries are scoped to the authenticated tenant.

---

## 6. Public SDK (`pkg/sdk`)

> **Added**: ERP Dogfood Refactor (Sep 2026)

Domain primitives that external modules and plugin authors need are exported via `pkg/sdk`. The `internal/core/domain/shared` package is now a thin alias layer pointing to `pkg/sdk`.

| Package | Types Exported |
|---|---|
| `pkg/sdk` | `ID`, `NewID()` (UUIDv7) |
| `pkg/sdk` | `Money`, `NewMoney()`, `MustMoney()` |
| `pkg/sdk` | `Quantity`, `NewQuantity()` |
| `pkg/sdk` | `AuthClaims`, `WithAuthClaims()`, `AuthClaimsFromContext()`, `HasPermission()` |
| `pkg/sdk` | `TenantID`, `WithTenantID()`, `RequireTenantID()` |
| `pkg/sdk` | `ErrUnauthorized`, `ErrForbidden`, `ErrNotFound`, `ErrConflict`, `ErrBadRequest` |
| `pkg/sdk` | `Version`, `BuildInfo` |

### Import rule
- Modules **import `pkg/sdk`**, never `internal/core/domain/shared` directly.
- Core internals may still use `internal/core/domain/shared` — it simply re-exports from `pkg/sdk`.

---

## 7. App Assembly (`pkg/app`)

`pkg/app` wires all server dependencies (database pool, repository instances, usecases, HTTP router) in one place, keeping `cmd/server/main.go` a thin entry point.

```go
// cmd/server/main.go
func main() {
    cfg := config.Load()
    application := app.New(app.Options{
        Config:        cfg,
        RunMigrations: true,  // run embedded SQL migrations on startup
    })
    application.Run()
}
```

`app.Options.RunMigrations = true` triggers `pkg/migrations` on startup, ensuring the schema is always up-to-date without requiring a separate migration step in CI/CD.

---

## 8. Embedded Migrations (`pkg/migrations`)

SQL migration files are embedded into the binary via `//go:embed` (Go 1.16+):

```go
// pkg/migrations/migrations.go
//go:embed sql/*.sql
var migrationFS embed.FS

func Up(ctx context.Context, pool *pgxpool.Pool) error { ... }
```

- Uses `github.com/golang-migrate/migrate/v4` with the `iofs` driver.
- Migration state is tracked in the `schema_migrations` table.
- Idempotent: re-running `Up()` on an already-migrated database is a no-op.

---

## 9. Bootstrap & Seed (`pkg/bootstrap`, `cmd/seed`)

System initialization (super-admin tenant, root organization, default roles/permissions) is handled by `pkg/bootstrap` and invocable via `make seed`.

```bash
make seed          # idempotent — safe to run multiple times
```

`cmd/seed` is a standalone binary separate from the HTTP server so seeding can be triggered in CI, Docker `entrypoint`, or Coolify deploy hooks without starting the API server.

---

## 10. Transactional Write Strategy (`pkg/database.WithTx`)

All aggregate mutations (document create/transition, comment, outbox event, audit log) are wrapped in a **single PostgreSQL transaction** via `database.WithTx`:

```go
// pkg/database/tx.go
func WithTx(ctx context.Context, pool *pgxpool.Pool, fn func(ctx context.Context) error) error
```

- If a `pgx.Tx` is already active in `ctx` (nested call), `fn` is called within the existing transaction (safe nesting).
- If no transaction is active, a new one is started, committed on success, or rolled back on any error/panic.
- Repositories detect the transaction via `TxFromContext(ctx)` and scope their `sqlc` queries accordingly.

### Invariant guaranteed by `WithTx`
> **Document state + audit log + outbox event are either all committed or all rolled back.** There is no partial write scenario.
