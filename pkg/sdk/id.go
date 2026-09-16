package sdk

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// ID is the universal domain identifier type, based on UUIDv7.
type ID = uuid.UUID

// NilID returns an empty/zero UUID.
func NilID() ID {
	return uuid.Nil
}

// NewID generates a new time-ordered UUIDv7.
func NewID() (ID, error) {
	return uuid.NewV7()
}

// MustNewID generates a new UUIDv7, panicking if system clock fails.
func MustNewID() ID {
	id, err := uuid.NewV7()
	if err != nil {
		panic(fmt.Sprintf("failed to generate UUIDv7: %v", err))
	}
	return id
}

// ParseID parses a string into an ID.
func ParseID(s string) (ID, error) {
	parsed, err := uuid.Parse(s)
	if err != nil {
		return NilID(), fmt.Errorf("%w: invalid UUID format", ErrInvalidInput)
	}
	return parsed, nil
}

// MustParseID parses a string into an ID or panics.
func MustParseID(s string) ID {
	id, err := ParseID(s)
	if err != nil {
		panic(err)
	}
	return id
}

// ToPgUUID converts sdk.ID to pgtype.UUID for pgx/sqlc compatibility.
func ToPgUUID(id ID) pgtype.UUID {
	return pgtype.UUID{
		Bytes: id,
		Valid: id != uuid.Nil,
	}
}

// FromPgUUID converts pgtype.UUID to sdk.ID.
func FromPgUUID(u pgtype.UUID) ID {
	if !u.Valid {
		return NilID()
	}
	return ID(u.Bytes)
}
