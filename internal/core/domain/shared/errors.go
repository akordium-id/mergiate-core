// Package shared re-exports sentinel errors from pkg/sdk.
// Internal code can continue to use shared.ErrNotFound etc. unchanged.
package shared

import "github.com/akordium-id/mergiate-core/pkg/sdk"

var (
	// Domain errors
	ErrNotFound      = sdk.ErrNotFound
	ErrAlreadyExists = sdk.ErrAlreadyExists
	ErrInvalidInput  = sdk.ErrInvalidInput
	ErrUnauthorized  = sdk.ErrUnauthorized
	ErrForbidden     = sdk.ErrForbidden
	ErrConflict      = sdk.ErrConflict
	ErrInternal      = sdk.ErrInternal

	// Multi-tenancy errors
	ErrTenantRequired  = sdk.ErrTenantRequired
	ErrTenantNotFound  = sdk.ErrTenantNotFound
	ErrTenantSuspended = sdk.ErrTenantSuspended

	// Value Object errors
	ErrCurrencyMismatch = sdk.ErrCurrencyMismatch
	ErrInvalidCurrency  = sdk.ErrInvalidCurrency
	ErrInvalidAmount    = sdk.ErrInvalidAmount
	ErrUnitMismatch     = sdk.ErrUnitMismatch
	ErrDivisionByZero   = sdk.ErrDivisionByZero
)
