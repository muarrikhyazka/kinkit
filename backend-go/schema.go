package main

import (
	"context"
	_ "embed"
)

//go:embed schema.sql
var schemaSQL string

// initSchema runs the embedded schema.sql against the pool on every boot.
// It must stay idempotent: new tables via CREATE TABLE IF NOT EXISTS, new
// columns via ALTER TABLE ... ADD COLUMN IF NOT EXISTS (see the header of
// schema.sql).
// A transaction-scoped advisory lock serializes concurrent boots (several
// replicas, parallel test packages) so IF NOT EXISTS checks can't race.
func initSchema(ctx context.Context, app *App) error {
	tx, err := app.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(727274)"); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, schemaSQL); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
