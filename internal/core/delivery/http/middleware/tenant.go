package middleware

import (
	"net/http"

	"github.com/akordium-id/mergiate-core/internal/core/domain/shared"
	"github.com/akordium-id/mergiate-core/pkg/response"
)

const HeaderTenantID = "X-Tenant-ID"

// TenantRequired enforces that a tenant context is established.
//
// Resolution order (after AuthRequired has run):
//  1. If a tenant ID was already injected into context by AuthRequired (from JWT/API-key claims),
//     that value is authoritative. If an X-Tenant-ID header is ALSO present and differs, the
//     request is rejected with 403 TENANT_MISMATCH to prevent cross-tenant access.
//  2. If no auth-injected tenant exists (unauthenticated path — should not reach here on
//     business routes guarded by AuthRequired), fall back to X-Tenant-ID header.
//  3. If neither is present, reject 400 TENANT_REQUIRED.
func TenantRequired() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claimsTenantID, hasClaims := shared.GetTenantID(r.Context())

			headerRaw := r.Header.Get(HeaderTenantID)

			if hasClaims && claimsTenantID != shared.NilID() {
				// Tenant injected by auth middleware — authoritative.
				// If a header is present and differs → cross-tenant attempt → 403.
				if headerRaw != "" {
					headerID, err := shared.ParseID(headerRaw)
					if err == nil && headerID != shared.NilID() && headerID != claimsTenantID {
						response.Err(w, http.StatusForbidden, "TENANT_MISMATCH",
							"X-Tenant-ID header does not match the authenticated tenant")
						return
					}
				}
				// Tenant already in context — proceed.
				next.ServeHTTP(w, r)
				return
			}

			// No auth-injected tenant: attempt header-only resolution.
			// This path should not be reached on routes guarded by AuthRequired.
			if headerRaw == "" {
				response.Err(w, http.StatusBadRequest, "TENANT_REQUIRED", "Missing X-Tenant-ID header")
				return
			}

			tenantID, err := shared.ParseID(headerRaw)
			if err != nil || tenantID == shared.NilID() {
				response.Err(w, http.StatusBadRequest, "INVALID_TENANT_ID", "X-Tenant-ID must be a valid UUID")
				return
			}

			ctx := shared.WithTenantID(r.Context(), tenantID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// TenantOptional parses the X-Tenant-ID header if present, but does not block requests if absent.
func TenantOptional() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rawID := r.Header.Get(HeaderTenantID)
			if rawID != "" {
				if tenantID, err := shared.ParseID(rawID); err == nil && tenantID != shared.NilID() {
					ctx := shared.WithTenantID(r.Context(), tenantID)
					r = r.WithContext(ctx)
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}
