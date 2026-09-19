package goals_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"homeqol/internal/db"
	"homeqol/internal/goals"
)

func newTestHandler(t *testing.T) http.Handler {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"), logger)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	repo := goals.NewRepository(database)
	svc := goals.NewService(repo, logger)
	return goals.NewRouter(svc)
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

func TestGoal_CreateAndGet(t *testing.T) {
	h := newTestHandler(t)

	rec := do(t, h, http.MethodPost, "/", map[string]any{"title": "Ship personal-os"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body)
	}
	var g goals.Goal
	json.NewDecoder(rec.Body).Decode(&g)
	if g.ID == "" {
		t.Fatal("expected non-empty ID")
	}

	rec2 := do(t, h, http.MethodGet, "/"+g.ID, nil)
	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec2.Code)
	}
}

func TestGoal_Create_validation(t *testing.T) {
	h := newTestHandler(t)
	rec := do(t, h, http.MethodPost, "/", map[string]any{"title": ""})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestGoal_List_empty(t *testing.T) {
	h := newTestHandler(t)
	rec := do(t, h, http.MethodGet, "/", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["goals"] == nil {
		t.Error("goals key must not be null")
	}
}

func TestGoal_Update(t *testing.T) {
	h := newTestHandler(t)
	rec := do(t, h, http.MethodPost, "/", map[string]any{"title": "Old"})
	var g goals.Goal
	json.NewDecoder(rec.Body).Decode(&g)

	rec2 := do(t, h, http.MethodPatch, "/"+g.ID, map[string]any{"title": "New"})
	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec2.Code, rec2.Body)
	}
	var updated goals.Goal
	json.NewDecoder(rec2.Body).Decode(&updated)
	if updated.Title != "New" {
		t.Errorf("title not updated: %q", updated.Title)
	}
}

func TestGoal_Delete(t *testing.T) {
	h := newTestHandler(t)
	rec := do(t, h, http.MethodPost, "/", map[string]any{"title": "Bye"})
	var g goals.Goal
	json.NewDecoder(rec.Body).Decode(&g)

	rec2 := do(t, h, http.MethodDelete, "/"+g.ID, nil)
	if rec2.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec2.Code)
	}
	rec3 := do(t, h, http.MethodGet, "/"+g.ID, nil)
	if rec3.Code != http.StatusNotFound {
		t.Errorf("expected 404 after delete, got %d", rec3.Code)
	}
}

func TestGoal_Get_404(t *testing.T) {
	h := newTestHandler(t)
	rec := do(t, h, http.MethodGet, "/ghost", nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestGoal_Milestone_isNestedUnderParent(t *testing.T) {
	h := newTestHandler(t)

	// Create parent goal.
	rec := do(t, h, http.MethodPost, "/", map[string]any{"title": "Parent"})
	var parent goals.Goal
	json.NewDecoder(rec.Body).Decode(&parent)

	// Create milestone under parent.
	rec2 := do(t, h, http.MethodPost, "/", map[string]any{
		"title":     "Milestone 1",
		"parent_id": parent.ID,
	})
	if rec2.Code != http.StatusCreated {
		t.Fatalf("expected 201 for milestone, got %d: %s", rec2.Code, rec2.Body)
	}

	// Get parent — should see milestone in Milestones slice.
	rec3 := do(t, h, http.MethodGet, "/"+parent.ID, nil)
	var fetched goals.Goal
	json.NewDecoder(rec3.Body).Decode(&fetched)
	if len(fetched.Milestones) != 1 {
		t.Errorf("expected 1 milestone, got %d", len(fetched.Milestones))
	}
}
