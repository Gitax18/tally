// Package migrations embeds the SQL migrations, so the binaries carry their own schema.
package migrations

import "embed"

// FS holds every *.sql migration, applied in version order.
//
//go:embed *.sql
var FS embed.FS
