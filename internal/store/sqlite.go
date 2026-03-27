package store

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	_ "modernc.org/sqlite" // SQLite driver
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// DB wraps a *sql.DB with Noto-specific helpers.
type DB struct {
	*sql.DB
}

// Open opens (or creates) the SQLite database at path and configures recommended PRAGMAs.
func Open(path string) (*DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("store: open %s: %w", path, err)
	}

	// Single writer connection is safest for SQLite.
	db.SetMaxOpenConns(1)

	if err := applyPragmas(db); err != nil {
		_ = db.Close()
		return nil, err
	}

	wrapped := &DB{DB: db}
	if err := wrapped.Migrate(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("store: migrate: %w", err)
	}

	return wrapped, nil
}

// applyPragmas sets recommended SQLite pragmas for reliability and performance.
func applyPragmas(db *sql.DB) error {
	pragmas := []string{
		`PRAGMA journal_mode = WAL`,
		`PRAGMA synchronous = NORMAL`,
		`PRAGMA foreign_keys = ON`,
		`PRAGMA busy_timeout = 5000`,
		`PRAGMA cache_size = -8000`, // 8 MB page cache
		`PRAGMA temp_store = MEMORY`,
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			return fmt.Errorf("store: pragma %q: %w", p, err)
		}
	}
	return nil
}

// Migrate applies all embedded SQL migration files in lexicographic order.
func (db *DB) Migrate() error {
	// Ensure schema_migrations table exists.
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version TEXT PRIMARY KEY,
			applied_at DATETIME NOT NULL DEFAULT (datetime('now'))
		)
	`); err != nil {
		return fmt.Errorf("store: create schema_migrations: %w", err)
	}

	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("store: read migrations dir: %w", err)
	}

	// Collect and sort migration file names.
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	for _, name := range files {
		version := strings.TrimSuffix(name, ".sql")

		var count int
		if err := db.QueryRow(
			`SELECT COUNT(*) FROM schema_migrations WHERE version = ?`, version,
		).Scan(&count); err != nil {
			return fmt.Errorf("store: check migration %s: %w", version, err)
		}
		if count > 0 {
			continue // already applied
		}

		data, err := migrationsFS.ReadFile("migrations/" + name)
		if err != nil {
			return fmt.Errorf("store: read migration %s: %w", name, err)
		}

		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("store: begin migration %s: %w", name, err)
		}
		if _, err := tx.Exec(string(data)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("store: exec migration %s: %w", name, err)
		}
		if _, err := tx.Exec(
			`INSERT INTO schema_migrations (version) VALUES (?)`, version,
		); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("store: record migration %s: %w", name, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("store: commit migration %s: %w", name, err)
		}
	}
	return nil
}

// WithTx executes fn inside a transaction. Rolls back on error, commits on success.
func (db *DB) WithTx(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: begin tx: %w", err)
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: commit tx: %w", err)
	}
	return nil
}
