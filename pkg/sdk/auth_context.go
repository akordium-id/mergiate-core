package sdk

import "context"

// ActorType identifies what kind of actor is making a request.
type ActorType string

const (
	ActorTypeUser   ActorType = "user"
	ActorTypeAPIKey ActorType = "api_key"
	ActorTypeSystem ActorType = "system"
)

// AuthClaims holds the authenticated user or service account credentials and RBAC permissions in context.
type AuthClaims struct {
	UserID           ID        `json:"user_id"`
	TenantID         ID        `json:"tenant_id"`
	Email            string    `json:"email"`
	Name             string    `json:"name"`
	Roles            []string  `json:"roles"`
	Permissions      []string  `json:"permissions"`
	ActorType        ActorType `json:"actor_type,omitempty"`
	ServiceAccountID *ID       `json:"service_account_id,omitempty"`
	APIKeyID         *ID       `json:"api_key_id,omitempty"`
}

// authContextKey is the type for the auth claims context key.
// Using a named type avoids collisions with keys from other packages.
// The string value is intentionally identical to what internal/core/domain/shared uses
// so that middleware setting via shared.WithAuthClaims and sdk.GetAuthClaims are interoperable.
type authContextKey string

const claimsContextKey authContextKey = "mergiate.auth_claims"

// WithAuthClaims stores the user's authentication claims in context.
func WithAuthClaims(ctx context.Context, claims *AuthClaims) context.Context {
	return context.WithValue(ctx, claimsContextKey, claims)
}

// GetAuthClaims retrieves authentication claims from context if present.
func GetAuthClaims(ctx context.Context) (*AuthClaims, bool) {
	val := ctx.Value(claimsContextKey)
	if val == nil {
		return nil, false
	}
	claims, ok := val.(*AuthClaims)
	return claims, ok && claims != nil
}

// HasPermission checks if the context claims contain a specific permission.
// The wildcard "*" permission grants access to all actions.
func HasPermission(ctx context.Context, permissionCode string) bool {
	claims, ok := GetAuthClaims(ctx)
	if !ok {
		return false
	}
	for _, p := range claims.Permissions {
		if p == permissionCode || p == "*" {
			return true
		}
	}
	return false
}
