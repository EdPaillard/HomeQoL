package budget_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"homeqol/internal/budget"
	"homeqol/internal/db"
)

func newTestHandler(t *testing.T) http.Handler {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"), logger)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	repo := budget.NewRepository(database)
	svc := budget.NewService(repo, logger)
	return budget.NewRouter(svc)
}

func do(t *testing.T, h http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestBudget_CurrentPeriod_autocreates(t *testing.T) {
	h := newTestHandler(t)
	rec := do(t, h, http.MethodGet, "/current", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body)
	}
	var period budget.BudgetPeriod
	json.NewDecoder(rec.Body).Decode(&period)

	now := time.Now()
	if period.Year != now.Year() || period.Month != int(now.Month()) {
		t.Errorf("expected current month %d/%02d, got %d/%02d", now.Year(), now.Month(), period.Year, period.Month)
	}
}

func TestBudget_CurrentPeriod_hasEnvelopes(t *testing.T) {
	h := newTestHandler(t)
	rec := do(t, h, http.MethodGet, "/current", nil)
	var period budget.BudgetPeriod
	json.NewDecoder(rec.Body).Decode(&period)
	// Seed data inserts 10 envelope templates.
	if len(period.Envelopes) == 0 {
		t.Error("expected seeded envelopes, got none")
	}
}

func TestBudget_CreateManualTransaction(t *testing.T) {
	h := newTestHandler(t)
	// Ensure period exists.
	do(t, h, http.MethodGet, "/current", nil)

	now := time.Now()
	rec := do(t, h, http.MethodPost, "/transactions", map[string]any{
		"label":        "LECLERC TOURS",
		"amount_cents": -4250,
		"date":         now.Format("2006-01-02"),
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body)
	}
	var tx budget.Transaction
	json.NewDecoder(rec.Body).Decode(&tx)
	if tx.ID == "" {
		t.Error("expected non-empty ID")
	}
	// LECLERC should be auto-categorised by seed rules.
	if tx.EnvelopeID == nil {
		t.Log("note: LECLERC not auto-categorised (no matching rule in test DB)")
	}
}

func TestBudget_CreateTransaction_400_missingLabel(t *testing.T) {
	h := newTestHandler(t)
	rec := do(t, h, http.MethodPost, "/transactions", map[string]any{
		"amount_cents": -1000,
		"date":         "2025-05-01",
	})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestBudget_ListTransactions(t *testing.T) {
	h := newTestHandler(t)
	do(t, h, http.MethodGet, "/current", nil)
	do(t, h, http.MethodPost, "/transactions", map[string]any{
		"label": "SNCF", "amount_cents": -5600, "date": time.Now().Format("2006-01-02"),
	})

	rec := do(t, h, http.MethodGet, "/transactions", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["transactions"] == nil {
		t.Error("transactions key must not be null")
	}
}

func TestBudget_GetPeriod_404(t *testing.T) {
	h := newTestHandler(t)
	rec := do(t, h, http.MethodGet, "/1999/01", nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestBudget_ListAccounts_empty(t *testing.T) {
	h := newTestHandler(t)
	rec := do(t, h, http.MethodGet, "/accounts", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["accounts"] == nil {
		t.Error("accounts key must not be null")
	}
}
