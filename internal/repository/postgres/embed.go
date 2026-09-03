package postgres

import "embed"

// migrationsFS embeds the SQL migration files used by goose.
//
// IMPORTANT: keep this directory in sync with /db/migrations at the project root.
// Whenever a new migration is added under db/migrations, copy it here as well so
// that the embedded binary contains the latest schema. The two locations must
// hold byte-identical files.

//go:embed migrations/*.sql
var migrationsFS embed.FS