// Package migrate applies the embedded SQL migrations (apps/api/db/migrations)
// against PostgreSQL, tracking what has already run in a schema_migrations
// table so re-running is a no-op.
package migrate

import (
	"context"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/HealthCarefund/carefund-platform/apps/api/db/migrations"
)

// Migration is one embedded, versioned SQL file.
type Migration struct {
	Version int64
	Name    string
	UpSQL   string
}

func loadMigrations() ([]Migration, error) {
	entries, err := fs.ReadDir(migrations.FS, ".")
	if err != nil {
		return nil, fmt.Errorf("reading embedded migrations: %w", err)
	}

	byVersion := map[int64]Migration{}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".up.sql") {
			continue
		}
		versionStr, rest, ok := strings.Cut(name, "_")
		if !ok {
			return nil, fmt.Errorf("migration file %q does not match the NNNN_name.up.sql convention", name)
		}
		version, err := strconv.ParseInt(versionStr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("migration file %q has a non-numeric version prefix: %w", name, err)
		}
		content, err := migrations.FS.ReadFile(name)
		if err != nil {
			return nil, fmt.Errorf("reading migration file %q: %w", name, err)
		}
		byVersion[version] = Migration{
			Version: version,
			Name:    strings.TrimSuffix(rest, ".up.sql"),
			UpSQL:   string(content),
		}
	}

	result := make([]Migration, 0, len(byVersion))
	for _, m := range byVersion {
		result = append(result, m)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Version < result[j].Version })
	return result, nil
}

// Run applies every migration not yet recorded in schema_migrations, each
// in its own transaction, in ascending version order. It returns the
// versions actually applied by this call (empty if already up to date).
func Run(ctx context.Context, pool *pgxpool.Pool) ([]int64, error) {
	if _, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version    BIGINT PRIMARY KEY,
			name       TEXT NOT NULL,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)
	`); err != nil {
		return nil, fmt.Errorf("ensuring schema_migrations table: %w", err)
	}

	all, err := loadMigrations()
	if err != nil {
		return nil, err
	}

	rows, err := pool.Query(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("reading applied migrations: %w", err)
	}
	appliedSet := map[int64]bool{}
	for rows.Next() {
		var v int64
		if err := rows.Scan(&v); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scanning applied migration version: %w", err)
		}
		appliedSet[v] = true
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating applied migrations: %w", err)
	}

	var applied []int64
	for _, m := range all {
		if appliedSet[m.Version] {
			continue
		}

		tx, err := pool.Begin(ctx)
		if err != nil {
			return applied, fmt.Errorf("beginning transaction for migration %d: %w", m.Version, err)
		}

		if _, err := tx.Exec(ctx, m.UpSQL); err != nil {
			_ = tx.Rollback(ctx)
			return applied, fmt.Errorf("applying migration %d (%s): %w", m.Version, m.Name, err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (version, name) VALUES ($1, $2)`, m.Version, m.Name); err != nil {
			_ = tx.Rollback(ctx)
			return applied, fmt.Errorf("recording migration %d: %w", m.Version, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return applied, fmt.Errorf("committing migration %d: %w", m.Version, err)
		}
		applied = append(applied, m.Version)
	}
	return applied, nil
}
