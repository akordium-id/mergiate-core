// Package shared re-exports public primitives from pkg/sdk for use within the
// internal implementation. External modules should import pkg/sdk directly.
//
// The context keys, sentinel errors, and ID type originate in pkg/sdk and are
// re-exported here so that internal packages can continue to use the shared.* names
// without any import changes.
package shared

import (
	"github.com/akordium-id/mergiate-core/pkg/sdk"
)

// ID is the universal domain identifier type (UUIDv7). Alias to sdk.ID.
type ID = sdk.ID

// ID helpers — delegated to sdk.
var (
	NilID       = sdk.NilID
	NewID       = sdk.NewID
	MustNewID   = sdk.MustNewID
	ParseID     = sdk.ParseID
	MustParseID = sdk.MustParseID
	ToPgUUID    = sdk.ToPgUUID
	FromPgUUID  = sdk.FromPgUUID
)
