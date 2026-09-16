// Package shared re-exports tenant context helpers from pkg/sdk.
package shared

import (
	"context"
	"time"

	"github.com/akordium-id/mergiate-core/pkg/sdk"
)

// TenantStatus represents the lifecycle status of a tenant.
type TenantStatus string

const (
	TenantStatusActive    TenantStatus = "active"
	TenantStatusSuspended TenantStatus = "suspended"
	TenantStatusArchived  TenantStatus = "archived"
)

// Tenant represents the top-level isolation and administrative boundary in Mergiate.
type Tenant struct {
	ID        ID             `json:"id"`
	Code      string         `json:"code"`
	Name      string         `json:"name"`
	Status    TenantStatus   `json:"status"`
	Settings  map[string]any `json:"settings"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

func (t Tenant) IsActive() bool {
	return t.Status == TenantStatusActive
}

// WithTenantID stores the Tenant ID in the given context.
// Delegates to sdk.WithTenantID so internal and external code share the same context key.
func WithTenantID(ctx context.Context, id ID) context.Context {
	return sdk.WithTenantID(ctx, id)
}

// GetTenantID retrieves the Tenant ID from the context if present.
func GetTenantID(ctx context.Context) (ID, bool) {
	return sdk.GetTenantID(ctx)
}

// RequireTenantID retrieves the Tenant ID from context, returning ErrTenantRequired if missing.
func RequireTenantID(ctx context.Context) (ID, error) {
	return sdk.RequireTenantID(ctx)
}
