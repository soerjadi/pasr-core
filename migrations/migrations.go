// Package migrations embeds the goose SQL migrations so tests and tools can apply them.
package migrations

import "embed"

// FS holds every *.sql migration file.
//
//go:embed *.sql
var FS embed.FS
