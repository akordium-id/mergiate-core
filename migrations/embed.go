// Package coremigrations embeds the core SQL migration files.
// This package is internal; use pkg/migrations for the public API.
package coremigrations

import "embed"

// FS contains all core migration .sql files.
//
//go:embed *.sql
var FS embed.FS
