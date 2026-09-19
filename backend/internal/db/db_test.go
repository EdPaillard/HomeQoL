package db_test

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"homeqol/internal/db"
)

func TestOpen_createsTablesOnFirstRun(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	d, err := db.Open(path, logger)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer d.Close()

	// Verify core tables exist by querying sqlite_master.
	tables := []string{
		"schema_migrations",
		"goals", "tasks", "tags", "task_tags",
		"habits", "habit_completions",
		"bank_accounts", "envelope_templates", "budget_periods",
		"budget_envelopes", "transactions", "categorisation_rules",
		"tydom_rooms", "tydom_temperature_readings",
		"tydom_shutter_readings", "tydom_sunlight_readings",
		"linky_energy_readings", "alerts",
	}

	for _, table := range tables {
		var name string
		err := d.QueryRow(
			`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table,
		).Scan(&name)
		if err != nil {
			t.Errorf("table %q not found after migration: %v", table, err)
		}
	}
}

func TestOpen_idempotentMigrations(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	// Opening twice should not fail — migrations are skipped on second open.
	for i := range 2 {
		d, err := db.Open(path, logger)
		if err != nil {
			t.Fatalf("Open #%d failed: %v", i+1, err)
		}
		d.Close()
	}
}

func TestOpen_seedDataInserted(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	d, err := db.Open(path, logger)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer d.Close()

	var count int
	if err := d.QueryRow(`SELECT COUNT(*) FROM envelope_templates`).Scan(&count); err != nil {
		t.Fatalf("query envelope_templates: %v", err)
	}
	if count == 0 {
		t.Error("expected seed envelope templates, got 0")
	}

	if err := d.QueryRow(`SELECT COUNT(*) FROM categorisation_rules`).Scan(&count); err != nil {
		t.Fatalf("query categorisation_rules: %v", err)
	}
	if count == 0 {
		t.Error("expected seed categorisation rules, got 0")
	}
}

func TestPing_healthCheck(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	d, err := db.Open(path, logger)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer d.Close()

	if result := d.Ping(); result != "" {
		t.Errorf("Ping() returned error: %s", result)
	}
}
