// Package migrations exposes the goose migration SQL files as an embed.FS,
// consumed by internal/pool's Migrate method.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
