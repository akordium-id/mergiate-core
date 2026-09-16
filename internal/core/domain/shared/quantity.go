// Package shared re-exports Quantity from pkg/sdk.
package shared

import "github.com/akordium-id/mergiate-core/pkg/sdk"

// Unit re-exported from sdk.
type Unit = sdk.Unit

// Quantity re-exported from sdk.
type Quantity = sdk.Quantity

// Quantity constructors re-exported from sdk.
var (
	NewQuantity     = sdk.NewQuantity
	MustNewQuantity = sdk.MustNewQuantity
)
