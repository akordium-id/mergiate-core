# Contributing to Mergiate Core

Thank you for your interest in contributing to **Mergiate Core**! As an open-core, modular platform for business applications, we follow strict architectural guidelines to keep the codebase clean, lean, performant, and maintainable.

Please take a few moments to review these guidelines before submitting an issue or pull request.

---

## 1. Core Architectural Invariants

Mergiate Core strictly enforces **7 Invariants** (detailed in [`docs/architecture.md`](docs/architecture.md)). Every contribution must adhere to them:

1. **Core Never Imports Modules**:
   - `core` $\to$ `modules` ❌ (Never import external module types or logic into core)
   - `modules` $\to$ `core` ✅ (Modules consume core primitives and contracts)
2. **Module Entities Never Masquerade as Core Primitives**:
   - Do not add specialized business objects (e.g., `SalesOrder`, `Employee`, `MedicalRecord`) into core domains.
   - Use Core `Document`, `Party`, `Product`, or Custom Fields instead.
3. **Multi-Tenancy is Inescapable**:
   - Every domain aggregate and database table (except global platforms/tenants) must have a `tenant_id`.
   - Never bypass `RequireTenantID(ctx)`.
4. **Transactions are Handled at Usecase Boundaries**:
   - Repositories do not start or commit transactions; usecases coordinate transaction units.
5. **Money and Quantities are Value Objects**:
   - Never use primitive `float64` for currency, monetary amounts, or units. Use `shared.Money` and `shared.Quantity`.
6. **Domain Events via Transactional Outbox**:
   - Changes that need asynchronous dispatch (webhooks, notification, event bus) must write to `outbox_events` atomically within the database transaction.
7. **Audit Trails are Immutable**:
   - Audit records are write-only. Never update or delete audit records.

---

## 2. Code Quality & Standards

- **Language & Style**:
  - Follow idiomatic Go guidelines (`effective_go`, Go Code Review Comments).
  - Code must pass `go vet ./...` without any warnings.
  - Run `go fmt ./...` or `goimports` on all changed files.
- **Language for Code & Documentation**:
  - All code identifiers, comments, commit messages, and documentation files must be written in **English**.
- **Error Handling**:
  - Use sentinel errors defined in `internal/core/domain/shared/errors.go` (e.g., `shared.ErrNotFound`, `shared.ErrInvalidInput`).
  - Always wrap errors with context: `fmt.Errorf("fetching party %s: %w", id, err)`.
- **Database Access (`sqlc` & `pgx`)**:
  - Write SQL queries in `internal/core/repository/postgres/queries/*.sql`.
  - Always generate code via `make sqlc`. Do not hand-edit files in `internal/core/repository/postgres/sqlc/`.
  - Avoid raw SQL strings in usecases or handlers.

---

## 3. Development Workflow

### Prerequisites
- **Go**: Version 1.26 or higher
- **Docker**: For running PostgreSQL 17 and Redis
- **Buf CLI**: If modifying protobuf contracts in `proto/`

### Local Setup
1. Fork and clone the repository.
2. Create your local environment file:
   ```bash
   cp .env.example .env
   ```
3. Start dependencies:
   ```bash
   make docker-up
   make migrate-up
   ```
4. Run tests:
   ```bash
   make test
   ```

---

## 4. Submitting a Pull Request (PR)

1. **Create a Feature Branch**:
   ```bash
   git checkout -b feat/your-feature-name
   # or fix/your-bug-fix
   ```
2. **Conventional Commits**:
   Format commit messages using [Conventional Commits](https://www.conventionalcommits.org/):
   - `feat(...)`: A new feature or capability
   - `fix(...)`: A bug fix
   - `perf(...)`: A code change that improves performance
   - `docs(...)`: Documentation only changes
   - `test(...)`: Adding or refactoring tests
   - `refactor(...)`: Code refactoring without changing behavior
3. **Verify Everything Locally Before Pushing**:
   ```bash
   go build ./...
   go vet ./...
   go test -v ./...
   ```
4. **Open a Pull Request**:
   - Provide a clear summary of the problem and your solution.
   - Reference any relevant GitHub Issue (e.g., `Fixes #12`).

---

## 5. Security & Responsible Disclosure

If you discover a security vulnerability within Mergiate Core, please do **NOT** open a public issue. Instead, report it privately to the maintainers at `security@akordium.id`.
