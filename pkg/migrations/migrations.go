// Package migrations provides embedded SQL migrations for mergiate-core and a
// programmatic migrator so external distro binaries (e.g. mergiate-erp) can
// apply the database schema without the golang-migrate CLI.
//
// Core migrations are embedded from the migrations/ directory at the module root
// via the internal coremigrations package.
// External (ERP-owned) migration sources can be passed via the extra variadic argument;
// they are expected to start at version 000100 to avoid colliding with core's 000001-…
package migrations

import (
	"errors"
	"fmt"
	"io/fs"
	"net/url"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5" // pgx driver
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"

	coremigrations "github.com/akordium-id/mergiate-core/migrations" // internal embed
)

// CoreFS is the embedded filesystem containing all core SQL migrations.
// External modules can read it directly if needed.
var CoreFS = coremigrations.FS

// Up applies all pending migrations.
// It first applies core's embedded migrations, then each extra FS in order.
// Passing an empty extra list runs only core migrations.
// The function is safe to call multiple times (idempotent — already-applied
// migrations are skipped by golang-migrate's version tracking).
func Up(pool *pgxpool.Pool, extra ...fs.FS) error {
	connStr := pool.Config().ConnString()

	// Apply core migrations first.
	if err := applyFS(CoreFS, ".", connStr); err != nil {
		return fmt.Errorf("core migrations: %w", err)
	}

	// Apply extra migration sources (e.g. ERP-owned).
	for i, extraFS := range extra {
		if err := applyFS(extraFS, ".", connStr); err != nil {
			return fmt.Errorf("extra migration source %d: %w", i, err)
		}
	}

	return nil
}

func applyFS(migrations fs.FS, dir, connStr string) error {
	src, err := iofs.New(migrations, dir)
	if err != nil {
		return fmt.Errorf("iofs source: %w", err)
	}

	u, err := url.Parse(connStr)
	if err != nil {
		return fmt.Errorf("parse conn str: %w", err)
	}
	u.Scheme = "pgx5"

	m, err := migrate.NewWithSourceInstance("iofs", src, u.String())
	if err != nil {
		return fmt.Errorf("migrate init: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}
