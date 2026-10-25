package migrate

import (
	"context"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"gofermart/migrations"

	"github.com/jackc/pgx/v5/pgxpool"
)

const BookkeepingTable = "schema_migrations"

const sqlSuffix = ".sql"

func Run(ctx context.Context, pool *pgxpool.Pool, fileSystem fs.FS, dir string) error {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("migrate: acquire connection: %w", err)
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, `CREATE TABLE IF NOT EXISTS `+BookkeepingTable+` (
		name TEXT PRIMARY KEY,
		applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
	)`); err != nil {
		return fmt.Errorf("migrate: create bookkeeping table: %w", err)
	}

	entries, err := fs.ReadDir(fileSystem, dir)
	if err != nil {
		return fmt.Errorf("migrate: read directory %s: %w", dir, err)
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), sqlSuffix) {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	for _, name := range names {
		var applied bool
		if err := conn.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM `+BookkeepingTable+` WHERE name = $1)`, name,
		).Scan(&applied); err != nil {
			return fmt.Errorf("migrate: check %s: %w", name, err)
		}
		if applied {
			continue
		}
		body, err := fs.ReadFile(fileSystem, name)
		if err != nil {
			return fmt.Errorf("migrate: read %s: %w", name, err)
		}
		tx, err := conn.Begin(ctx)
		if err != nil {
			return fmt.Errorf("migrate: begin %s: %w", name, err)
		}

		if _, err := tx.Exec(ctx, string(body)); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("migrate: apply %s: %w", name, err)
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO `+BookkeepingTable+` (name) VALUES ($1)`, name,
		); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("migrate: record %s: %w", name, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("migrate: commit %s: %w", name, err)
		}
	}
	return nil
}

func RunEmbedded(ctx context.Context, pool *pgxpool.Pool) error {
	return Run(ctx, pool, migrations.FS, ".")
}
