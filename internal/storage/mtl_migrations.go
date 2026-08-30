package storage

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strconv"
	"strings"
)

const mtlMigrationsPath = "migrations/mtl"

//go:embed migrations/mtl/*.sql
var mtlMigrationsFS embed.FS

// MigrateMTL applies independently versioned Montelibero overlay migrations.
func (p *SQLProvider) MigrateMTL(ctx context.Context) (err error) {
	tx, err := p.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin MTL schema migration: %w", err)
	}

	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if _, err = tx.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS mtl_schema_migrations (
    version INTEGER PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);`); err != nil {
		return fmt.Errorf("failed to create MTL schema migration history: %w", err)
	}

	var current int
	if err = tx.GetContext(ctx, &current, `SELECT COALESCE(MAX(version), 0) FROM mtl_schema_migrations`); err != nil {
		return fmt.Errorf("failed to read MTL schema version: %w", err)
	}

	migrations, err := loadMTLMigrations(current)
	if err != nil {
		return err
	}

	for _, migration := range migrations {
		if _, err = tx.ExecContext(ctx, migration.query); err != nil {
			return fmt.Errorf("failed to apply MTL schema migration %d (%s): %w", migration.version, migration.name, err)
		}

		query := tx.Rebind(`INSERT INTO mtl_schema_migrations (version, name) VALUES (?, ?)`)
		if _, err = tx.ExecContext(ctx, query, migration.version, migration.name); err != nil {
			return fmt.Errorf("failed to record MTL schema migration %d: %w", migration.version, err)
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit MTL schema migration: %w", err)
	}

	return nil
}

type mtlMigration struct {
	version int
	name    string
	query   string
}

func loadMTLMigrations(current int) (migrations []mtlMigration, err error) {
	entries, err := fs.ReadDir(mtlMigrationsFS, mtlMigrationsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read MTL schema migrations: %w", err)
	}

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".up.sql") {
			continue
		}

		parts := strings.SplitN(name, "_", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid MTL schema migration name %q", name)
		}

		version, parseErr := strconv.Atoi(parts[0])
		if parseErr != nil {
			return nil, fmt.Errorf("invalid MTL schema migration version in %q: %w", name, parseErr)
		}

		if version <= current {
			continue
		}

		query, readErr := mtlMigrationsFS.ReadFile(path.Join(mtlMigrationsPath, name))
		if readErr != nil {
			return nil, fmt.Errorf("failed to read MTL schema migration %q: %w", name, readErr)
		}

		migrations = append(migrations, mtlMigration{
			version: version,
			name:    strings.TrimSuffix(parts[1], ".up.sql"),
			query:   string(query),
		})
	}

	sort.Slice(migrations, func(i, j int) bool { return migrations[i].version < migrations[j].version })

	return migrations, nil
}
