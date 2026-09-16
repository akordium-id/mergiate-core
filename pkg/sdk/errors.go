package sdk

import "errors"

var (
	// Domain errors
	ErrNotFound      = errors.New("resource not found")
	ErrAlreadyExists = errors.New("resource already exists")
	ErrInvalidInput  = errors.New("invalid input data")
	ErrUnauthorized  = errors.New("unauthorized")
	ErrForbidden     = errors.New("forbidden")
	ErrConflict      = errors.New("resource conflict")
	ErrInternal      = errors.New("internal server error")

	// Multi-tenancy errors
	ErrTenantRequired  = errors.New("tenant context is required")
	ErrTenantNotFound  = errors.New("tenant not found")
	ErrTenantSuspended = errors.New("tenant is suspended")

	// Value Object errors
	ErrCurrencyMismatch = errors.New("currency mismatch between money operations")
	ErrInvalidCurrency  = errors.New("invalid currency code")
	ErrInvalidAmount    = errors.New("invalid money amount")
	ErrUnitMismatch     = errors.New("unit mismatch between quantity operations")
	ErrDivisionByZero   = errors.New("division by zero")
)
