package habits_test

import (
	"log/slog"
	"path/filepath"
	"testing"

	"homeqol/internal/db"
)

// openTestDB opens a temporary SQLite database for tests.
// The database is automatically cleaned up when the test finishes.
// Using a file path (not :memory:) ensures the go-sqlite3 WAL mode works
// correctly and migrations run identically to production.
func openTestDB(t *testing.T, logger *slog.Logger) *db.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	database, err := db.Open(path, logger)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}
