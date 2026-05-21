// Package migrations bundles the SQL migration files into the binary via
// embed.FS so the migrate subcommand is self-contained — no host filesystem
// dependency, no separate volume mount needed in K8s.
//
// To add a migration: drop a new file in this directory (or use
// `go run ./cmd/migrate create <name> sql` to scaffold one). The build picks
// it up automatically.
package migrations

import "embed"

// FS holds all .sql migration files in this directory, baked into the binary.
//
//go:embed *.sql
var FS embed.FS
