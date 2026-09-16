# MASTER PROMPT
# mergiate-core — Security Hardening, Public SDK, App Assembly & Bootstrap

## 0. ROLE

You are a senior Go engineer and platform architect working **inside the `mergiate-core` repository**.

Your task is to prepare `mergiate-core` to become the foundation layer of a separate ERP
application (**mergiate-erp** + a Nuxt frontend) that a different team/agent is building
in parallel. mergiate-core must become:

1. **Secure** — every business endpoint authenticated and permission-checked server-side.
2. **Importable** — a public SDK that an external Go module can build modules against.
3. **Embeddable** — a public "app assembly" API so an external distro binary can mount
   all core routes plus its own modules in one process.
4. **Bootable** — an idempotent bootstrap/seed mechanism so a fresh install is usable.

Do NOT build any ERP business features (CRM, Sales, Projects, Finance) in this repo.
Your scope is platform only.

---

## 1. CONTEXT

Current stack (verify, don't assume): Go 1.26, Chi v5, pgx/v5 + sqlc, PostgreSQL 17,
JWT HS256 (golang-jwt/v5), bcrypt, transactional-outbox-to-in-memory-bus, module SPI in
`pkg/module`. Clean Architecture: `internal/core/{domain,usecase,repository/postgres,delivery/http}`.

An architecture audit (already performed; verify each claim before acting on it) found:

- `middleware.RequirePermission`/`RequireAnyPermission` are applied **only** in
  `identity_handler.go` (IAM routes). All other resource handlers (`documents`, `parties`,
  `products`, `units`, `audit-logs`, `custom-fields`, `sequences`, `files`, `attachments`,
  `comments`, `notifications`, `activities`) apply only `TenantRequired()` — no auth.
- `/api/v1/tenants` CRUD is **fully unauthenticated**.
- `TenantRequired()` trusts a client-supplied `X-Tenant-ID` header even for JWT users,
  without validating membership → cross-tenant access if a tenant UUID is known.
- CORS: `AllowedOrigins: ["*"]` combined with `AllowCredentials: true` (invalid per spec).
- `pkg/auth` falls back to an insecure default JWT secret when the configured secret is empty.
- `Host` (in `pkg/module/host.go`) references `internal/core/domain/shared.ID` and
  `internal/core/domain/event.OutboxRepository` → external Go modules cannot compile
  against this repo. This blocks the ERP repo and is the reason for the SDK workstream.
- Document/audit/outbox writes are not in one DB transaction; outbox/audit insert errors
  are discarded with `_ =` in `document_usecase.go` and `communication_usecase.go`.
- No bootstrap/seed: a fresh install has no tenant, no organization, no user, no roles.
  Only the `permissions` catalog is seeded by migrations.

---

## 2. GROUND RULES

1. Work on a feature branch. Keep `go test ./...` green at every commit.
2. Write tests for every new behavior before or with the change (repo style: stdlib
   `testing`, table-driven + `t.Run`, handwritten in-memory fakes; `testify` exists but
   is used sparingly — prefer stdlib style).
3. Follow existing patterns exactly: sqlc for new SQL (add queries under
   `internal/core/repository/postgres/queries/`, run `make sqlc`), usecase interface +
   `NewUsecase` constructor, `pkg/response` envelopes, `shared.*` context helpers.
4. **Smallest correct change.** Do not refactor beyond the workstreams below.
5. Verify every audit claim against the code before acting; if reality differs, adapt
   and note it in your final report.
6. Migrations are **additive only**. Never edit existing migration files (they may have
   been applied). New migrations start at `000014`.

---

## 3. WORKSTREAM A — SECURITY HARDENING (priority 0)

### A1. Route guard matrix

Apply authentication + permission middleware to every route group. Permission codes
already exist in the seeded catalog (colon convention `domain:resource:action`). Where a
needed code is missing, add it to the seeded catalog (new migration, `ON CONFLICT DO
NOTHING`).

| Route group | Middleware | Permission for non-GET | Permission for GET |
|---|---|---|---|
| `/auth/*` | unchanged (register/login public; me/switch-tenant `AuthRequired`) | — | — |
| `/tenants` write (POST, PUT) | `AuthRequired` | `tenant:manage` (NEW, seed it) | — |
| `/tenants` read (list, get-by-id, get-by-code) | `AuthRequired` | — | membership in target tenant OR `tenant:manage` OR `*` |
| `/organizations` | `AuthRequired` + `TenantRequired` | `organization:manage` (NEW, seed) | `organization:read` (NEW, seed) |
| `/parties` (+roles/addresses/contacts) | `AuthRequired` + `TenantRequired` | `party:create`/`party:update`/`party:delete` | `party:read` |
| `/units`, `/units/conversions`, `/units/convert` | `AuthRequired` + `TenantRequired` | `product:create` | `product:read` |
| `/products` (+variants) | `AuthRequired` + `TenantRequired` | `product:create`/`product:update`/`product:delete` (seed `product:delete` if missing) | `product:read` |
| `/documents` (+transition) | `AuthRequired` + `TenantRequired` | `document:create` / `document:transition` | `document:read` |
| `/audit-logs` | `AuthRequired` + `TenantRequired` | — | `audit:read` |
| `/custom-fields/*` | `AuthRequired` + `TenantRequired` | `custom_field:manage` | `custom_field:manage` (read defintions) — or add `custom_field:read` |
| `/sequences*` | `AuthRequired` + `TenantRequired` | `sequence:manage` | `sequence:manage` |
| `/files/*`, `/attachments*` | `AuthRequired` + `TenantRequired` | `file:upload` / `file:delete` / `attachment:manage` | `file:read` |
| `/comments`, `/notifications`, `/activities` | `AuthRequired` + `TenantRequired` | `comment:create` / `comment:delete` | `comment:read` / `notification:read` |
| `/iam/*` | unchanged (already guarded) | | |
| `/service-accounts*`, `/service-accounts/{id}/keys*` | `AuthRequired` + enforce existing `service_account:*` / `api_key:*` codes | | |
| `/modules` (discovery) | `AuthRequired` | — | `iam:manage` OR `*` |

Rules:
- `AuthRequired` (not `AuthOptional`) everywhere above. Anonymous → `401`.
- Permission failure → `403` with the standard envelope.
- Module routes mounted under `/api/v1/modules/{name}` must also be behind
  `AuthRequired`; module-scoped permission checks stay the module's responsibility,
  but unauthenticated access must be impossible.
- Keep the wildcard `*` super-admin bypass semantics of `HasPermission`.

### A2. Tenant resolution hardening

For an authenticated identity (user JWT or API key):
- The effective tenant **must come from claims** (JWT `tid` / API-key injected tenant).
- If an `X-Tenant-ID` header is present and differs from the claims tenant, reject with
  `403` (envelope error code `TENANT_MISMATCH`). Do not silently honor the header.
- Header-only tenant resolution (no identity) is no longer acceptable on any `/api/v1`
  business route — remove that path or gate it behind an explicit dev-only config flag
  that defaults to off.
- `TenantRequired()` reads the tenant from `shared.GetTenantID(ctx)` (set by
  `AuthRequired`) after the change above.

### A3. CORS configuration

- Add `CORS_ALLOWED_ORIGINS` env (comma-separated). Empty ⇒ CORS middleware effectively
  allows nothing cross-origin (same-origin/Nuxt-proxy deployments don't need it).
- Never combine wildcard origins with `AllowCredentials`. Dev default may be
  `http://localhost:3000`.

### A4. JWT secret hygiene

- If `APP_ENV=production` (or `production`-like) and the JWT secret is empty or equals
  the known insecure fallback → **refuse to start** with a clear error.
- In non-production, log a loud warning when the fallback is in use.

### A5. Tests (acceptance gate for A)

`httptest`-based matrix tests (no DB needed where middleware is testable in isolation;
use fakes):
- anonymous `GET /api/v1/documents` → 401
- valid JWT, no header, tenant from claims → passes guard
- valid JWT + `X-Tenant-ID` of another tenant → 403 `TENANT_MISMATCH`
- valid JWT lacking `document:create` → `POST /api/v1/documents` → 403
- JWT with `*` → allowed
- API key path still works through the unified middleware
- `/tenants` POST without `tenant:manage` → 403; anonymous → 401

---

## 4. WORKSTREAM B — PUBLIC SDK EXTRACTION (priority 0)

Goal: an external Go module (`mergiate-erp`) must be able to import this repo and build
modules against public API only. Strategy: **move nothing that would churn call sites** —
create public packages and keep `internal` types as aliases where useful.

### B1. `internal/core/domain/shared` → `pkg/sdk`

Move the value objects and context machinery into `internal/core/domain/shared`-shaped
public package `pkg/sdk`:

- `sdk.ID` (= `uuid.UUID`), `NewID`, `MustNewID`, `ParseID`, `ToPgUUID`, `FromPgUUID`
- `sdk.Money` (int64 minor units + currency), `NewMoney`, `MustNewMoney`, `ZeroMoney`,
  arithmetic + predicates, JSON shape unchanged
- `sdk.Quantity`
- `sdk.AuthClaims`, `sdk.ActorType` (user | api_key | system), `WithAuthClaims` /
  `GetAuthClaims` / `HasPermission` (wildcard `*`)
- tenant context: `WithTenantID` / `GetTenantID` / `RequireTenantID`
- the sentinel error taxonomy (`ErrNotFound`, `ErrAlreadyExists`, `ErrInvalidInput`,
  `ErrUnauthorized`, `ErrForbidden`, `ErrConflict`, `ErrTenantRequired`,
  `ErrTenantNotFound`, `ErrTenantSuspended`, `ErrCurrencyMismatch`, `ErrInvalidCurrency`,
  `ErrInvalidAmount`, `ErrUnitMismatch`, `ErrDivisionByZero`)

Then in `internal/core/domain/shared` keep **type aliases** (`type ID = sdk.ID`,
`func NewID() = sdk.NewID` shims or re-exported vars) so the entire internal tree keeps
compiling with minimal diffs. It is acceptable (often cleaner) to flip internal imports
to `pkg/sdk` mechanically — choose whichever yields the smallest, safest diff, but the
public result is the same.

Do NOT move aggregate types (Document, Party, Product...). Those stay internal. Only
primitives, auth context, and errors become public.

### B2. `pkg/module` cleanup

`Host` must reference only public/vendor types:

```go
type Host interface {
    DB() *pgxpool.Pool
    EventBus() eventbus.Bus
    Storage() storage.Driver
    TokenManager() auth.TokenManager
    Logger() *slog.Logger
    RecordOutbox(ctx context.Context, tenantID sdk.ID, eventType, aggregateType string,
        aggregateID sdk.ID, payload map[string]any) error
    // Consider adding (optional, for modules that follow repo conventions):
    // Audit(ctx, actorID *sdk.ID, actorType, action, entityType string, entityID sdk.ID, changes, metadata map[string]any) error
}
```

(`event.OutboxRepository` usage moves inside the implementation.)

### B3. Version marker

Add a `sdk.Version` constant (or `pkg/sdk/doc.go` note) and bump awareness in the
module SPI docs. Optional: tag the repo (e.g. `sdk-v0.1.0`) when done.

---

## 5. WORKSTREAM C — APP ASSEMBLY API `pkg/app` (priority 0)

An external distro binary must be able to run core + its own modules in one process.
Expose (naming may be refined, semantics may not):

```go
package app // pkg/app

type Options struct {
    Config        *config.Config      // existing pkg/config, extended with CORS_ALLOWED_ORIGINS
    Logger        *slog.Logger
    Modules       []module.Module     // registered before router build
    RunMigrations bool                // if true, run pkg/migrations.Up on start
    ExtraMigrationFS []fs.FS          // module-owned migration sources to apply after core's
}

func New(ctx context.Context, opts Options) (*App, error)
  // connects DB pool, wires ALL repositories/usecases/handlers currently wired in
  // cmd/server/main.go, applies security middleware (Workstream A), initializes the
  // module registry (InitAll, RegisterPermissions, BindSubscriptions)

func (a *App) Router() chi.Router        // /healthz, /readyz, /api/v1 (core + /modules/*)
func (a *App) Host() module.Host         // for advanced embedding
func (a *App) DB() *pgxpool.Pool
func (a *App) Tokens() auth.TokenManager
func (a *App) Logger() *slog.Logger
func (a *App) Start(ctx context.Context) error   // outbox worker + http server
func (a *App) Shutdown(ctx context.Context) error // graceful, reverse-order module shutdown
```

- `cmd/server/main.go` must be rewritten to be a thin wrapper over `pkg/app` (same
  behavior as today, plus the security workstream). No behavior duplication elsewhere.
- Module routes stay mounted under `/api/v1/modules/{name}` and behind `AuthRequired`.

---

## 6. WORKSTREAM D — BOOTSTRAP / SEED (priority 0)

Idempotent bootstrap so a fresh install is immediately usable:

1. New command `cmd/seed` + `make seed` (env-driven; also callable as
   `seed -email ... -password ... -tenant ... -org ...`).
2. Creates, if missing (natural-key idempotent, safe to re-run):
   - Tenant (code, e.g. `akordium`; settings `{"locale":"id_ID","currency":"IDR"}`)
   - Organization (name, type `company`) within that tenant
   - Owner user (email + bcrypt password from env/flags)
   - System roles (per-tenant, `is_system=true`):
     - `owner` → permission `*` (or all seeded codes; prefer explicit list for auditability, plus a real `*` role is acceptable — pick one, document it)
     - `admin`, `finance`, `sales`, `project_manager`, `staff` → reasonable subsets of the
       catalog (staff read-only across business domains; document the mapping in the seed source)
   - `user_roles` link owner→`owner`
3. New seeded permissions this workstream: `tenant:manage`, `organization:manage`,
   `organization:read`, `product:delete`, plus anything the A-matrix identified as missing.
4. Prints a summary of what it created/skipped. Exit 0 when everything already exists.
5. Test: idempotency (run twice → second run creates nothing), role-permission mapping
   correctness.

---

## 7. WORKSTREAM E — EMBEDDED MIGRATIONS + PROGRAMMATIC MIGRATOR (priority 1)

So the ERP distro can migrate the DB without the golang-migrate CLI and without a second
version-tracking conflict:

1. `pkg/migrations` with `//go:embed migrations/*.sql` (both up & down).
2. `pkg/migrations.Up(pool *pgxpool.Pool, extra ...fs.FS) error` — programmatic
   golang-migrate (add `github.com/golang-migrate/migrate/v4` with postgres + iofs
   drivers) applying core's embedded migrations first, then each `extra` source
   (ERP-owned, expected to start at version `000100` to never collide with core's
   `000001–…`).
3. `app.Options.RunMigrations` + `ExtraMigrationFS` hook into this (Workstream C).
4. Keep `make migrate-up` (CLI) working for local dev.

---

## 8. WORKSTREAM F — TRANSACTIONAL OUTBOX & AUDIT (priority 2 — do last)

Today `document_usecase`/`communication_usecase` write the aggregate, then write
audit/outbox rows in **separate** operations with errors discarded (`_ =`). Fix so the
aggregate write + transitions + audit + outbox happen in **one DB transaction**:

- Introduce the smallest viable pattern (e.g. a `WithTx(ctx, fn)` helper on repositories
  or a usecase-level transaction coordinator with pgx `BeginFunc`).
- Errors from audit/outbox writes must abort the transaction (that's the point).
- Keep function signatures stable where possible; it's fine to change internal usecase
  internals and repo interfaces.
- Test: simulate outbox insert failure → document creation must roll back.

---

## 9. NON-GOALS (hard)

- Do NOT implement CRM/Sales/Projects/Finance or any business module in this repo.
- Do NOT change the JSON response envelope or existing field names/shapes of existing
  endpoints. (Endpoints gaining 401/403 is expected and desired.)
- Do NOT rename existing permission codes (`document:create`, `party:read`, ...).
- Do NOT add refresh tokens, password reset, 2FA, rate limiting in this pass — out of
  scope; note them as future work.
- Do NOT introduce new frameworks or replace Chi/pgx/sqlc.
- Do NOT touch `pkg/eventbus` semantics (sync, in-memory stays).

---

## 10. PUBLIC CONTRACT (the ERP side codes against this — do not drift)

- Module path stays `github.com/akordium-id/mergiate-core`.
- `pkg/sdk`: primitives, auth context, tenant context, sentinel errors (names as today,
  relocated).
- `pkg/module`: `Module`, `Manifest`, `PermissionDefinition`, `Subscription`, `Host`
  (cleaned per B2). Module routes mount at `/api/v1/modules/{manifest.Name}`,
  discovery at `GET /api/v1/modules`.
- `pkg/app`: assembly API per section 5.
- `pkg/migrations`: embed + `Up(pool, extra ...fs.FS)` per section 7.
- `pkg/auth`, `pkg/config`, `pkg/database`, `pkg/response`, `pkg/storage`,
  `pkg/storage/local`: unchanged APIs.
- Permission convention: `domain:resource:action`, colon-separated.
- Auth headers: `Authorization: Bearer <jwt>` or `X-API-Key`; `X-Tenant-ID` only
  validated against claims (see A2).
- Seeded system roles after `make seed`: `owner, admin, finance, sales, project_manager, staff`.

---

## 11. TESTING REQUIREMENTS

- All existing tests stay green (`go test ./...`).
- New: security matrix (A5), seed idempotency, `pkg/app` assembly smoke test (build
  router with a stub module; hit discovery endpoint; assert 401 anonymously, 200 with
  token), migrations `Up` idempotency against an ephemeral DB if practical (skip with
  `t.Skip` when no DB is available).
- `make sqlc generate` output committed when queries change.

---

## 12. IMPLEMENTATION WORKFLOW

1. Verify the audit claims in section 1 against the code (quick, route-by-route).
2. Land workstreams in order: A → B → C → D → E → F. Each lands green.
3. After each workstream: `go vet ./... && go test ./...`, run the server once
   (`make run`) and curl the health + one guarded endpoint to sanity-check.
4. Finish with a short report: what changed per workstream, any audit claim that turned
   out wrong, new env vars (`CORS_ALLOWED_ORIGINS`, seed vars), and any contract note
   the ERP side must know.

---

## 13. DEFINITION OF DONE

- [ ] Every `/api/v1` business route returns 401 anonymously and enforces its permission.
- [ ] Tenant always derives from authenticated claims; cross-tenant header → 403.
- [ ] CORS configurable, no wildcard+credentials.
- [ ] Production refuses insecure JWT secret.
- [ ] `pkg/sdk` exists; `pkg/module.Host` compiles from outside the module (test by a
      throwaway external package or `go build` in a temp module with a `replace`).
- [ ] `pkg/app` runs core + a stub module in one process.
- [ ] `make seed` produces a working login for the owner user with full access.
- [ ] `pkg/migrations.Up` applies core migrations programmatically and accepts extra FS.
- [ ] Document+audit+outbox in one transaction (F).
- [ ] `go test ./...` green; new tests added per section 11.
