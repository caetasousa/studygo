package db

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// migration is one ordered pair of up/down SQL scripts.
type migration struct {
	version int
	name    string
	up      string
}

// migrateLockID is an arbitrary constant for the session-level advisory lock
// that serializes migration runs across the server and worker processes.
const migrateLockID = 8_274_411_903

// Migrate applies every pending ".up.sql" migration found in fsys, in filename
// order, inside a transaction each, and records them in schema_migrations. It is
// safe to call concurrently from multiple processes: a Postgres advisory lock
// serializes the runs, and already-applied versions are skipped.
func Migrate(ctx context.Context, pool *pgxpool.Pool, fsys fs.FS) error {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquiring migration connection: %w", err)
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock($1)`, migrateLockID); err != nil {
		return fmt.Errorf("acquiring migration lock: %w", err)
	}
	defer conn.Exec(context.WithoutCancel(ctx), `SELECT pg_advisory_unlock($1)`, migrateLockID)

	if _, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version    integer     PRIMARY KEY,
			name       text        NOT NULL,
			applied_at timestamptz NOT NULL DEFAULT now()
		)`); err != nil {
		return fmt.Errorf("creating schema_migrations: %w", err)
	}

	migrations, err := loadMigrations(fsys)
	if err != nil {
		return err
	}

	applied, err := appliedVersions(ctx, pool)
	if err != nil {
		return err
	}

	for _, m := range migrations {
		if applied[m.version] {
			continue
		}

		if err := applyOne(ctx, pool, m); err != nil {
			return fmt.Errorf("applying migration %06d_%s: %w", m.version, m.name, err)
		}
	}

	return nil
}

func applyOne(ctx context.Context, pool *pgxpool.Pool, m migration) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, m.up); err != nil {
		return fmt.Errorf("exec: %w", err)
	}

	if _, err := tx.Exec(
		ctx,
		`INSERT INTO schema_migrations (version, name) VALUES ($1, $2)`,
		m.version,
		m.name,
	); err != nil {
		return fmt.Errorf("recording version: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	return nil
}

func appliedVersions(ctx context.Context, pool *pgxpool.Pool) (map[int]bool, error) {
	rows, err := pool.Query(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("querying applied versions: %w", err)
	}
	defer rows.Close()

	applied := map[int]bool{}

	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return nil, fmt.Errorf("scanning version: %w", err)
		}

		applied[v] = true
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating versions: %w", err)
	}

	return applied, nil
}

// loadMigrations reads every "NNNNNN_name.up.sql" entry from fsys.
func loadMigrations(fsys fs.FS) ([]migration, error) {
	entries, err := fs.Glob(fsys, "*.up.sql")
	if err != nil {
		return nil, fmt.Errorf("globbing migrations: %w", err)
	}

	sort.Strings(entries)

	migrations := make([]migration, 0, len(entries))

	for _, entry := range entries {
		version, name, err := parseName(entry)
		if err != nil {
			return nil, err
		}

		content, err := fs.ReadFile(fsys, entry)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", entry, err)
		}

		migrations = append(migrations, migration{
			version: version,
			name:    name,
			up:      string(content),
		})
	}

	return migrations, nil
}

func parseName(entry string) (int, string, error) {
	base := strings.TrimSuffix(entry, ".up.sql")

	idx := strings.IndexByte(base, '_')
	if idx <= 0 {
		return 0, "", fmt.Errorf("migration %q must be named NNNNNN_name.up.sql", entry)
	}

	var version int
	if _, err := fmt.Sscanf(base[:idx], "%d", &version); err != nil {
		return 0, "", fmt.Errorf("migration %q has non-numeric version: %w", entry, err)
	}

	return version, base[idx+1:], nil
}

// ErrNoRows re-exports pgx.ErrNoRows so adapters can match it without importing
// pgx directly through the port layer.
var ErrNoRows = pgx.ErrNoRows

// IsNoRows reports whether err is a "no rows" result.
func IsNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}
