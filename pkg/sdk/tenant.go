package sdk

import "context"

// tenantContextKey is the type for the tenant ID context key.
// String value is intentionally identical to internal/core/domain/shared for interoperability.
type tenantContextKey string

const tenantIDContextKey tenantContextKey = "mergiate.tenant_id"

// WithTenantID stores the Tenant ID in the given context.
func WithTenantID(ctx context.Context, id ID) context.Context {
	return context.WithValue(ctx, tenantIDContextKey, id)
}

// GetTenantID retrieves the Tenant ID from the context if present.
func GetTenantID(ctx context.Context) (ID, bool) {
	val := ctx.Value(tenantIDContextKey)
	if val == nil {
		return NilID(), false
	}
	id, ok := val.(ID)
	if !ok || id == NilID() {
		return NilID(), false
	}
	return id, true
}

// RequireTenantID retrieves the Tenant ID from context, returning ErrTenantRequired if missing.
func RequireTenantID(ctx context.Context) (ID, error) {
	id, ok := GetTenantID(ctx)
	if !ok {
		return NilID(), ErrTenantRequired
	}
	return id, nil
}
