// Package migrations embeds the hand-written SQL migration files so the runner
// can apply them without the files being present on disk at runtime.
package migrations

import "embed"

// FS holds every ".up.sql" / ".down.sql" file in this directory.
//
//go:embed *.sql
var FS embed.FS
