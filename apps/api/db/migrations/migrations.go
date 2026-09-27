// Package migrations embeds the SQL migration files so the built binary
// can apply them without needing the source tree on disk.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
