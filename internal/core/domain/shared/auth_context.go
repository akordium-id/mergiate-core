// Package shared re-exports auth context helpers from pkg/sdk.
package shared

import (
	"context"

	"github.com/akordium-id/mergiate-core/pkg/sdk"
)

// ActorType re-exported from sdk.
type ActorType = sdk.ActorType

const (
	ActorTypeUser   = sdk.ActorTypeUser
	ActorTypeAPIKey = sdk.ActorTypeAPIKey
	ActorTypeSystem = sdk.ActorTypeSystem
)

// AuthClaims re-exported from sdk.
type AuthClaims = sdk.AuthClaims

// WithAuthClaims stores the user's authentication claims in context.
// Delegates to sdk.WithAuthClaims so internal and external code share the same context key.
func WithAuthClaims(ctx context.Context, claims *AuthClaims) context.Context {
	return sdk.WithAuthClaims(ctx, claims)
}

// GetAuthClaims retrieves authentication claims from context if present.
func GetAuthClaims(ctx context.Context) (*AuthClaims, bool) {
	return sdk.GetAuthClaims(ctx)
}

// HasPermission checks if the context claims contain a specific permission.
func HasPermission(ctx context.Context, permissionCode string) bool {
	return sdk.HasPermission(ctx, permissionCode)
}
