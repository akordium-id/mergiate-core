// Package bootstrap provides idempotent seeding functionality for mergiate-core.
// It initializes the default tenant, organization, owner user, system roles,
// and role-permission mappings.
package bootstrap

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/akordium-id/mergiate-core/internal/core/domain/identity"
	"github.com/akordium-id/mergiate-core/internal/core/domain/organization"
	"github.com/akordium-id/mergiate-core/internal/core/domain/shared"
	"github.com/akordium-id/mergiate-core/internal/core/repository/postgres"
	"github.com/akordium-id/mergiate-core/pkg/auth"
)

// Config configures the bootstrap execution parameters.
type Config struct {
	Email      string
	Password   string
	Name       string
	TenantCode string
	TenantName string
	OrgCode    string
	OrgName    string
}

// Result reports what was created vs skipped during the bootstrap run.
type Result struct {
	TenantCreated   bool
	TenantID        shared.ID
	OrgCreated      bool
	OrgID           shared.ID
	UserCreated     bool
	UserID          shared.ID
	MemberCreated   bool
	RolesCreated    int
	RolesSkipped    int
	UserRoleCreated bool
}

// Summary returns a human-readable summary of actions taken.
func (r *Result) Summary() string {
	var sb strings.Builder
	sb.WriteString("==================================================\n")
	sb.WriteString("Mergiate Core Bootstrap Summary\n")
	sb.WriteString("==================================================\n")

	if r.TenantCreated {
		sb.WriteString(fmt.Sprintf("Tenant:       CREATED (ID: %s)\n", r.TenantID))
	} else {
		sb.WriteString(fmt.Sprintf("Tenant:       SKIPPED (already exists: %s)\n", r.TenantID))
	}

	if r.OrgCreated {
		sb.WriteString(fmt.Sprintf("Organization: CREATED (ID: %s)\n", r.OrgID))
	} else {
		sb.WriteString(fmt.Sprintf("Organization: SKIPPED (already exists: %s)\n", r.OrgID))
	}

	if r.UserCreated {
		sb.WriteString(fmt.Sprintf("Owner User:   CREATED (ID: %s)\n", r.UserID))
	} else {
		sb.WriteString(fmt.Sprintf("Owner User:   SKIPPED (already exists: %s)\n", r.UserID))
	}

	if r.MemberCreated {
		sb.WriteString("Membership:   CREATED\n")
	} else {
		sb.WriteString("Membership:   SKIPPED (already member)\n")
	}

	sb.WriteString(fmt.Sprintf("System Roles: %d created, %d existing\n", r.RolesCreated, r.RolesSkipped))

	if r.UserRoleCreated {
		sb.WriteString("Owner Role:   ASSIGNED to owner user\n")
	} else {
		sb.WriteString("Owner Role:   SKIPPED (already assigned)\n")
	}

	sb.WriteString("==================================================\n")
	return sb.String()
}

// Run executes the idempotent bootstrap process.
func Run(ctx context.Context, pool *pgxpool.Pool, cfg Config) (*Result, error) {
	if cfg.Email == "" {
		return nil, fmt.Errorf("email is required")
	}
	if cfg.Password == "" {
		return nil, fmt.Errorf("password is required")
	}
	if cfg.TenantCode == "" {
		cfg.TenantCode = "akordium"
	}
	if cfg.TenantName == "" {
		cfg.TenantName = "Akordium Lab"
	}
	if cfg.OrgCode == "" {
		cfg.OrgCode = "main"
	}
	if cfg.OrgName == "" {
		cfg.OrgName = "Akordium Main Org"
	}
	if cfg.Name == "" {
		cfg.Name = "Owner Administrator"
	}

	tenantRepo := postgres.NewTenantRepository(pool)
	orgRepo := postgres.NewOrganizationRepository(pool)
	identityRepo := postgres.NewIdentityRepository(pool)

	result := &Result{}

	// Ensure wildcard '*' permission exists in permissions table
	_, _ = pool.Exec(ctx, `
		INSERT INTO permissions (id, code, name, category, description)
		VALUES (gen_random_uuid(), '*', 'Superuser Wildcard', 'system', 'Wildcard permission granting full system access')
		ON CONFLICT (code) DO NOTHING;
	`)

	// 1. Tenant
	tenant, err := tenantRepo.GetByCode(ctx, cfg.TenantCode)
	if err != nil && err != shared.ErrNotFound {
		return nil, fmt.Errorf("check tenant %s: %w", cfg.TenantCode, err)
	}

	if tenant == nil {
		tID, err := shared.NewID()
		if err != nil {
			return nil, err
		}
		newTenant := &shared.Tenant{
			ID:     tID,
			Code:   cfg.TenantCode,
			Name:   cfg.TenantName,
			Status: shared.TenantStatusActive,
			Settings: map[string]any{
				"locale":   "id_ID",
				"currency": "IDR",
			},
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}
		if err := tenantRepo.Create(ctx, newTenant); err != nil {
			return nil, fmt.Errorf("create tenant: %w", err)
		}
		tenant = newTenant
		result.TenantCreated = true
	}
	result.TenantID = tenant.ID

	// 2. Organization within Tenant
	org, err := orgRepo.GetByCode(ctx, tenant.ID, cfg.OrgCode)
	if err != nil && err != shared.ErrNotFound {
		return nil, fmt.Errorf("check org %s: %w", cfg.OrgCode, err)
	}

	if org == nil {
		oID, err := shared.NewID()
		if err != nil {
			return nil, err
		}
		newOrg := &organization.Organization{
			ID:        oID,
			TenantID:  tenant.ID,
			Code:      cfg.OrgCode,
			Name:      cfg.OrgName,
			Type:      organization.TypeCompany,
			Status:    organization.StatusActive,
			Settings:  map[string]any{},
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}
		if err := orgRepo.Create(ctx, newOrg); err != nil {
			return nil, fmt.Errorf("create organization: %w", err)
		}
		org = newOrg
		result.OrgCreated = true
	}
	result.OrgID = org.ID

	// 3. Owner User
	user, err := identityRepo.GetUserByEmail(ctx, strings.ToLower(cfg.Email))
	if err != nil && err != shared.ErrNotFound {
		return nil, fmt.Errorf("check user %s: %w", cfg.Email, err)
	}

	if user == nil {
		hashed, err := auth.HashPassword(cfg.Password)
		if err != nil {
			return nil, fmt.Errorf("hash password: %w", err)
		}
		uID, err := shared.NewID()
		if err != nil {
			return nil, err
		}
		newUser := &identity.User{
			ID:           uID,
			Email:        strings.ToLower(cfg.Email),
			PasswordHash: hashed,
			Name:         cfg.Name,
			Status:       identity.UserStatusActive,
			CreatedAt:    time.Now().UTC(),
			UpdatedAt:    time.Now().UTC(),
		}
		if err := identityRepo.CreateUser(ctx, newUser); err != nil {
			return nil, fmt.Errorf("create user: %w", err)
		}
		user = newUser
		result.UserCreated = true
	}
	result.UserID = user.ID

	// 4. Tenant Membership
	member, err := identityRepo.GetTenantMember(ctx, tenant.ID, user.ID)
	if err != nil && err != shared.ErrNotFound {
		return nil, fmt.Errorf("check membership: %w", err)
	}

	if member == nil {
		mID, err := shared.NewID()
		if err != nil {
			return nil, err
		}
		newMember := &identity.TenantUser{
			ID:       mID,
			TenantID: tenant.ID,
			UserID:   user.ID,
			Status:   identity.MembershipStatusActive,
			JoinedAt: time.Now().UTC(),
		}
		if err := identityRepo.AddTenantMember(ctx, newMember); err != nil {
			return nil, fmt.Errorf("add tenant member: %w", err)
		}
		result.MemberCreated = true
	}

	// 5. System Roles
	allPermissions, err := identityRepo.ListPermissions(ctx)
	if err != nil {
		return nil, fmt.Errorf("list permissions: %w", err)
	}

	permMap := make(map[string]shared.ID, len(allPermissions))
	var allPermIDs []shared.ID
	var nonTenantManagePermIDs []shared.ID

	for _, p := range allPermissions {
		permMap[p.Code] = p.ID
		allPermIDs = append(allPermIDs, p.ID)
		if p.Code != "tenant:manage" && p.Code != "*" {
			nonTenantManagePermIDs = append(nonTenantManagePermIDs, p.ID)
		}
	}

	// Define system roles with their descriptors and permission lists
	type roleDef struct {
		code        string
		name        string
		description string
		getPermIDs  func() []shared.ID
	}

	rolesToSeed := []roleDef{
		{
			code:        "owner",
			name:        "Owner",
			description: "Tenant owner with full system privileges (* wildcard and all permissions)",
			getPermIDs: func() []shared.ID {
				return allPermIDs
			},
		},
		{
			code:        "admin",
			name:        "Administrator",
			description: "Tenant administrator with full operational privileges (except tenant:manage)",
			getPermIDs: func() []shared.ID {
				return nonTenantManagePermIDs
			},
		},
		{
			code:        "finance",
			name:        "Finance Manager",
			description: "Finance management: document lifecycle, audit logs, sequences, and comments",
			getPermIDs: func() []shared.ID {
				codes := []string{
					"document:create", "document:read", "document:update", "document:transition", "document:delete",
					"audit:read", "sequence:manage",
					"file:read", "file:upload", "file:delete", "attachment:manage",
					"comment:create", "comment:read", "comment:delete", "notification:read",
				}
				return resolveCodes(codes, permMap)
			},
		},
		{
			code:        "sales",
			name:        "Sales Representative",
			description: "Sales management: parties, products, document creation, comments, and attachments",
			getPermIDs: func() []shared.ID {
				codes := []string{
					"party:create", "party:read", "party:update", "party:delete",
					"product:read",
					"document:create", "document:read",
					"attachment:manage", "file:read", "file:upload",
					"comment:create", "comment:read", "notification:read",
				}
				return resolveCodes(codes, permMap)
			},
		},
		{
			code:        "project_manager",
			name:        "Project Manager",
			description: "Project manager: documents, parties, audit logs, comments, and attachments",
			getPermIDs: func() []shared.ID {
				codes := []string{
					"document:create", "document:read", "document:update", "document:transition", "document:delete",
					"party:read", "audit:read",
					"comment:create", "comment:read", "comment:delete",
					"attachment:manage", "file:read", "file:upload", "file:delete",
					"notification:read",
				}
				return resolveCodes(codes, permMap)
			},
		},
		{
			code:        "staff",
			name:        "Staff Member",
			description: "Staff read-only privileges across business domains",
			getPermIDs: func() []shared.ID {
				codes := []string{
					"document:read", "party:read", "product:read", "organization:read",
					"audit:read", "file:read", "comment:read", "notification:read",
				}
				return resolveCodes(codes, permMap)
			},
		},
	}

	var ownerRoleID shared.ID

	for _, rd := range rolesToSeed {
		role, err := identityRepo.GetRoleByCode(ctx, tenant.ID, rd.code)
		if err != nil && err != shared.ErrNotFound {
			return nil, fmt.Errorf("check role %s: %w", rd.code, err)
		}

		if role == nil {
			rID, err := shared.NewID()
			if err != nil {
				return nil, err
			}
			newRole := &identity.Role{
				ID:          rID,
				TenantID:    tenant.ID,
				Code:        rd.code,
				Name:        rd.name,
				Description: rd.description,
				IsSystem:    true,
				CreatedAt:   time.Now().UTC(),
				UpdatedAt:   time.Now().UTC(),
			}
			if err := identityRepo.CreateRole(ctx, newRole); err != nil {
				return nil, fmt.Errorf("create role %s: %w", rd.code, err)
			}
			role = newRole
			result.RolesCreated++

			// Assign permissions
			permIDs := rd.getPermIDs()
			if err := identityRepo.AssignPermissionsToRole(ctx, role.ID, permIDs); err != nil {
				return nil, fmt.Errorf("assign permissions to %s: %w", rd.code, err)
			}
		} else {
			result.RolesSkipped++
			// Sync permissions if role has none
			if len(role.Permissions) == 0 {
				permIDs := rd.getPermIDs()
				_ = identityRepo.AssignPermissionsToRole(ctx, role.ID, permIDs)
			}
		}

		if rd.code == "owner" {
			ownerRoleID = role.ID
		}
	}

	// 6. Assign Owner Role to Owner User
	userRoles, err := identityRepo.ListUserRolesInTenant(ctx, tenant.ID, user.ID)
	if err != nil {
		return nil, fmt.Errorf("list user roles: %w", err)
	}

	hasOwnerRole := false
	for _, ur := range userRoles {
		if ur.Code == "owner" {
			hasOwnerRole = true
			break
		}
	}

	if !hasOwnerRole && ownerRoleID != shared.NilID() {
		if err := identityRepo.AssignUserRole(ctx, tenant.ID, user.ID, ownerRoleID); err != nil {
			return nil, fmt.Errorf("assign owner role to user: %w", err)
		}
		result.UserRoleCreated = true
	}

	return result, nil
}

func resolveCodes(codes []string, permMap map[string]shared.ID) []shared.ID {
	var ids []shared.ID
	for _, c := range codes {
		if id, ok := permMap[c]; ok {
			ids = append(ids, id)
		}
	}
	return ids
}
