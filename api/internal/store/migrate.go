package store

import (
	"context"
	"embed"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// Migrate applies embedded SQL migrations sequentially, tracking progress
// in a schema_migrations table.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	const createTable = `CREATE TABLE IF NOT EXISTS schema_migrations (
		version    INT PRIMARY KEY,
		applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
	)`
	if _, err := pool.Exec(ctx, createTable); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	var current int
	row := pool.QueryRow(ctx, `SELECT COALESCE(MAX(version), 0) FROM schema_migrations`)
	if err := row.Scan(&current); err != nil {
		return fmt.Errorf("read current migration version: %w", err)
	}

	migrations, err := loadMigrations()
	if err != nil {
		return err
	}
	for _, m := range migrations {
		if m.version <= current {
			continue
		}
		if err := applyMigration(ctx, pool, m); err != nil {
			return err
		}
	}
	return nil
}

type migration struct {
	version int
	name    string
	sql     string
}

func loadMigrations() ([]migration, error) {
	entries, err := migrationFS.ReadDir("migrations")
	if err != nil {
		return nil, fmt.Errorf("read migrations dir: %w", err)
	}
	migrations := make([]migration, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		version, err := migrationVersion(name)
		if err != nil {
			return nil, err
		}
		content, err := migrationFS.ReadFile("migrations/" + name)
		if err != nil {
			return nil, fmt.Errorf("read migration %s: %w", name, err)
		}
		migrations = append(migrations, migration{version: version, name: name, sql: string(content)})
	}
	sort.Slice(migrations, func(i, j int) bool { return migrations[i].version < migrations[j].version })
	return migrations, nil
}

func migrationVersion(name string) (int, error) {
	prefix, _, found := strings.Cut(name, "_")
	if !found {
		return 0, fmt.Errorf("migration %s: expected <version>_<name>.sql", name)
	}
	version, err := strconv.Atoi(prefix)
	if err != nil {
		return 0, fmt.Errorf("migration %s: invalid version prefix: %w", name, err)
	}
	return version, nil
}

func applyMigration(ctx context.Context, pool *pgxpool.Pool, m migration) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin migration %s: %w", m.name, err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, m.sql); err != nil {
		return fmt.Errorf("apply migration %s: %w", m.name, err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, m.version); err != nil {
		return fmt.Errorf("record migration %s: %w", m.name, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit migration %s: %w", m.name, err)
	}
	return nil
}
