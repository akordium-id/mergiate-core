package migrations_test

import (
	"context"
	"io/fs"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/akordium-id/mergiate-core/pkg/migrations"
)

func TestCoreFSContainsMigrations(t *testing.T) {
	entries, err := fs.ReadDir(migrations.CoreFS, ".")
	if err != nil {
		t.Fatalf("failed to read CoreFS: %v", err)
	}

	if len(entries) == 0 {
		t.Fatal("CoreFS is empty, expected embedded SQL migrations")
	}

	has0001Up := false
	has0014Up := false
	for _, entry := range entries {
		if entry.Name() == "000001_init_tenants.up.sql" {
			has0001Up = true
		}
		if entry.Name() == "000014_seed_new_permissions.up.sql" {
			has0014Up = true
		}
	}

	if !has0001Up {
		t.Errorf("expected 000001_init_tenants.up.sql to be present in CoreFS")
	}
	if !has0014Up {
		t.Errorf("expected 000014_seed_new_permissions.up.sql to be present in CoreFS")
	}
}

func TestUpWithDatabase(t *testing.T) {
	connStr := "postgres://mergiate:mergiate_password@localhost:5434/mergiate_core?sslmode=disable"
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		t.Skipf("skipping database test: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		t.Skipf("database unreachable, skipping: %v", err)
	}

	if err := migrations.Up(pool); err != nil {
		t.Fatalf("migrations.Up failed: %v", err)
	}

	// Verify idempotency: second Up should succeed with ErrNoChange handled cleanly
	if err := migrations.Up(pool); err != nil {
		t.Fatalf("second migrations.Up (idempotency) failed: %v", err)
	}
}
