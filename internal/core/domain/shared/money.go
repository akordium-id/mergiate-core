// Package shared re-exports Money from pkg/sdk.
package shared

import "github.com/akordium-id/mergiate-core/pkg/sdk"

// Money re-exported from sdk.
type Money = sdk.Money

// Money constructors and helpers re-exported from sdk.
var (
	NewMoney    = sdk.NewMoney
	MustNewMoney = sdk.MustNewMoney
	ZeroMoney   = sdk.ZeroMoney
)
