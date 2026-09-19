package db

import (
	"database/sql"
	"embed"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3" // CGO SQLite driver
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// DB wraps *sql.DB with project-specific helpers.
type DB struct {
	*sql.DB
	logger *slog.Logger
	path   string
}

// Open opens (or creates) the SQLite database at the given path,
// applies all pending migrations, and returns a ready-to-use DB.
//
// Call Close() when the application shuts down.
func Open(path string, logger *slog.Logger) (*DB, error) {
	// Ensure the directory exists (e.g. /app/data/)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create db directory: %w", err)
	}

	// The DSN includes several important pragmas:
	//   _foreign_keys=ON  — enforce FK constraints (off by default in SQLite)
	//   _journal_mode=WAL — Write-Ahead Logging: better concurrent read performance
	//   _busy_timeout=5000 — wait up to 5s if the DB is locked (e.g. backup running)
	//   _synchronous=NORMAL — good balance of durability vs performance with WAL
	dsn := fmt.Sprintf(
		"file:%s?_foreign_keys=ON&_journal_mode=WAL&_busy_timeout=5000&_synchronous=NORMAL",
		path,
	)

	sqlDB, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	// SQLite works best with a single writer. Allow multiple readers.
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	sqlDB.SetConnMaxLifetime(0) // connections live forever (no network timeout needed)

	d := &DB{DB: sqlDB, logger: logger, path: path}

	if err := d.ping(); err != nil {
		return nil, err
	}
	if err := d.migrate(); err != nil {
		return nil, err
	}

	return d, nil
}

// Ping checks that the database connection is alive. Used by the /ready handler.
// Returns "" on success, error description on failure.
func (d *DB) Ping() string {
	if err := d.DB.Ping(); err != nil {
		return err.Error()
	}
	return ""
}

// ping is the internal version that returns an error (used during Open).
func (d *DB) ping() error {
	if err := d.DB.Ping(); err != nil {
		return fmt.Errorf("ping sqlite: %w", err)
	}
	return nil
}

// migrate runs all SQL migration files in migrations/ that haven't been
// applied yet, in filename order (001, 002, …).
//
// Each migration runs in its own transaction. If any statement fails,
// the transaction is rolled back and migration stops — the database is
// left in the last known-good state.
func (d *DB) migrate() error {
	// Ensure the tracking table exists before we query it.
	// This is the bootstrapping step — it's idempotent (IF NOT EXISTS).
	_, err := d.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		filename   TEXT PRIMARY KEY,
		applied_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
	)`)
	if err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	// Read the embedded migration files.
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}

	// Sort by filename — guarantees 001 < 002 < 003 …
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}

		filename := entry.Name()

		// Check if already applied.
		var count int
		err := d.QueryRow(
			`SELECT COUNT(*) FROM schema_migrations WHERE filename = ?`, filename,
		).Scan(&count)
		if err != nil {
			return fmt.Errorf("check migration %s: %w", filename, err)
		}
		if count > 0 {
			d.logger.Debug("migration already applied, skipping", slog.String("file", filename))
			continue
		}

		// Read the SQL file.
		content, err := migrationsFS.ReadFile("migrations/" + filename)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", filename, err)
		}

		// Apply in a transaction.
		d.logger.Info("applying migration", slog.String("file", filename))
		start := time.Now()

		if err := d.applyMigration(filename, string(content)); err != nil {
			return fmt.Errorf("apply migration %s: %w", filename, err)
		}

		d.logger.Info("migration applied",
			slog.String("file", filename),
			slog.String("duration", time.Since(start).String()),
		)
	}

	return nil
}

// applyMigration runs a single SQL file inside a transaction and records it.
func (d *DB) applyMigration(filename, sql string) error {
	tx, err := d.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback() // no-op if Commit() succeeds

	// SQLite's Exec accepts multiple statements separated by semicolons.
	if _, err := tx.Exec(sql); err != nil {
		return fmt.Errorf("execute sql: %w", err)
	}

	// Record that this migration was applied.
	if _, err := tx.Exec(
		`INSERT INTO schema_migrations (filename) VALUES (?)`, filename,
	); err != nil {
		return fmt.Errorf("record migration: %w", err)
	}

	return tx.Commit()
}

// Stats returns database size and connection pool stats for /ready diagnostics.
func (d *DB) Stats() map[string]any {
	stats := d.DB.Stats()
	return map[string]any{
		"open_connections": stats.OpenConnections,
		"in_use":           stats.InUse,
		"idle":             stats.Idle,
		"path":             d.path,
	}
}
