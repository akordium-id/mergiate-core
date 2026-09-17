package bootstrap_test

import (
	"context"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/akordium-id/mergiate-core/internal/core/domain/shared"
	"github.com/akordium-id/mergiate-core/internal/core/repository/postgres"
	"github.com/akordium-id/mergiate-core/pkg/bootstrap"
	"github.com/akordium-id/mergiate-core/pkg/migrations"
)

func TestBootstrapIdempotencyAndPermissions(t *testing.T) {
	connStr := "postgres://mergiate:mergiate_password@localhost:5434/mergiate_core?sslmode=disable"
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		t.Skipf("skipping bootstrap test: database pool creation failed: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		t.Skipf("skipping bootstrap test: database unreachable: %v", err)
	}

	// Ensure migrations are applied
	if err := migrations.Up(pool); err != nil {
		t.Fatalf("migrations.Up failed: %v", err)
	}

	cfg := bootstrap.Config{
		Email:      "test-owner@akordium.com",
		Password:   "AdminSecret123!",
		Name:       "Test Owner",
		TenantCode: "test-akordium",
		TenantName: "Test Akordium Lab",
		OrgCode:    "test-main",
		OrgName:    "Test Main Org",
	}

	// Cleanup test artifacts before starting
	cleanupTestArtifacts(t, pool, cfg)
	defer cleanupTestArtifacts(t, pool, cfg)

	// --- RUN 1: Fresh bootstrap ---
	res1, err := bootstrap.Run(ctx, pool, cfg)
	if err != nil {
		t.Fatalf("run 1 failed: %v", err)
	}

	if !res1.TenantCreated {
		t.Errorf("expected tenant to be created on run 1")
	}
	if !res1.OrgCreated {
		t.Errorf("expected org to be created on run 1")
	}
	if !res1.UserCreated {
		t.Errorf("expected user to be created on run 1")
	}
	if !res1.MemberCreated {
		t.Errorf("expected membership to be created on run 1")
	}
	if res1.RolesCreated != 6 {
		t.Errorf("expected 6 roles created on run 1, got %d", res1.RolesCreated)
	}
	if !res1.UserRoleCreated {
		t.Errorf("expected owner user role to be created on run 1")
	}

	// --- RUN 2: Idempotent re-run ---
	res2, err := bootstrap.Run(ctx, pool, cfg)
	if err != nil {
		t.Fatalf("run 2 failed: %v", err)
	}

	if res2.TenantCreated {
		t.Errorf("expected tenant to be skipped on run 2")
	}
	if res2.OrgCreated {
		t.Errorf("expected org to be skipped on run 2")
	}
	if res2.UserCreated {
		t.Errorf("expected user to be skipped on run 2")
	}
	if res2.MemberCreated {
		t.Errorf("expected membership to be skipped on run 2")
	}
	if res2.RolesCreated != 0 {
		t.Errorf("expected 0 roles created on run 2, got %d", res2.RolesCreated)
	}
	if res2.RolesSkipped != 6 {
		t.Errorf("expected 6 roles skipped on run 2, got %d", res2.RolesSkipped)
	}
	if res2.UserRoleCreated {
		t.Errorf("expected user role link to be skipped on run 2")
	}

	// --- ROLE-PERMISSION VERIFICATION ---
	identityRepo := postgres.NewIdentityRepository(pool)

	// 1. Verify owner permissions include wildcard '*'
	ownerRole, err := identityRepo.GetRoleByCode(ctx, res1.TenantID, "owner")
	if err != nil {
		t.Fatalf("failed to fetch owner role: %v", err)
	}
	hasWildcard := false
	for _, p := range ownerRole.Permissions {
		if p.Code == "*" {
			hasWildcard = true
			break
		}
	}
	if !hasWildcard {
		t.Errorf("owner role should have '*' permission")
	}

	// 2. Verify admin permissions DO NOT include 'tenant:manage'
	adminRole, err := identityRepo.GetRoleByCode(ctx, res1.TenantID, "admin")
	if err != nil {
		t.Fatalf("failed to fetch admin role: %v", err)
	}
	for _, p := range adminRole.Permissions {
		if p.Code == "tenant:manage" {
			t.Errorf("admin role must NOT have 'tenant:manage' permission")
		}
	}

	// 3. Verify staff permissions are read-only
	staffRole, err := identityRepo.GetRoleByCode(ctx, res1.TenantID, "staff")
	if err != nil {
		t.Fatalf("failed to fetch staff role: %v", err)
	}
	if len(staffRole.Permissions) == 0 {
		t.Errorf("staff role should have read permissions")
	}
	for _, p := range staffRole.Permissions {
		if p.Code != "document:read" && p.Code != "party:read" && p.Code != "product:read" &&
			p.Code != "organization:read" && p.Code != "audit:read" && p.Code != "file:read" &&
			p.Code != "comment:read" && p.Code != "notification:read" {
			t.Errorf("staff role has unexpected permission: %s", p.Code)
		}
	}

	// 4. Verify user permissions in tenant include '*'
	perms, err := identityRepo.ListUserPermissionsInTenant(ctx, res1.TenantID, res1.UserID)
	if err != nil {
		t.Fatalf("failed to list user permissions: %v", err)
	}
	userHasWildcard := slices.Contains(perms, "*")
	if !userHasWildcard {
		t.Errorf("owner user should have '*' permission in tenant")
	}
}

func cleanupTestArtifacts(t *testing.T, pool *pgxpool.Pool, cfg bootstrap.Config) {
	ctx := context.Background()
	// Clean up by tenant code
	tenantRepo := postgres.NewTenantRepository(pool)
	tenant, err := tenantRepo.GetByCode(ctx, cfg.TenantCode)
	if err == nil && tenant != nil {
		_, _ = pool.Exec(ctx, "DELETE FROM tenants WHERE id = $1", shared.ToPgUUID(tenant.ID))
	}

	// Clean up user by email
	_, _ = pool.Exec(ctx, "DELETE FROM users WHERE email = $1", cfg.Email)
}
