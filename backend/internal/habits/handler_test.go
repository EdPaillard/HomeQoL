package habits_test

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

	"homeqol/internal/db"
	"homeqol/internal/habits"
)

func newTestHandler(t *testing.T) http.Handler {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"), logger)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	repo := habits.NewRepository(database)
	svc := habits.NewService(repo, logger, time.Local)
	return habits.NewRouter(svc)
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

func TestHabit_Create_200(t *testing.T) {
	h := newTestHandler(t)
	rec := do(t, h, http.MethodPost, "/", map[string]any{"title": "Morning run"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body)
	}
	var habit habits.Habit
	json.NewDecoder(rec.Body).Decode(&habit)
	if habit.ID == "" {
		t.Error("expected non-empty ID")
	}
}

func TestHabit_Create_400_missingTitle(t *testing.T) {
	h := newTestHandler(t)
	rec := do(t, h, http.MethodPost, "/", map[string]any{"frequency": "daily"})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestHabit_List(t *testing.T) {
	h := newTestHandler(t)
	do(t, h, http.MethodPost, "/", map[string]any{"title": "Read"})
	do(t, h, http.MethodPost, "/", map[string]any{"title": "Meditate"})

	rec := do(t, h, http.MethodGet, "/", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	count := resp["count"].(float64)
	if count != 2 {
		t.Errorf("expected count=2, got %.0f", count)
	}
}

func TestHabit_Get_404(t *testing.T) {
	h := newTestHandler(t)
	rec := do(t, h, http.MethodGet, "/ghost", nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestHabit_Complete_201(t *testing.T) {
	h := newTestHandler(t)
	rec := do(t, h, http.MethodPost, "/", map[string]any{"title": "Exercise"})
	var habit habits.Habit
	json.NewDecoder(rec.Body).Decode(&habit)

	rec2 := do(t, h, http.MethodPost, "/"+habit.ID+"/complete", nil)
	if rec2.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec2.Code, rec2.Body)
	}
}

func TestHabit_Complete_409_duplicate(t *testing.T) {
	h := newTestHandler(t)
	rec := do(t, h, http.MethodPost, "/", map[string]any{"title": "Read"})
	var habit habits.Habit
	json.NewDecoder(rec.Body).Decode(&habit)

	do(t, h, http.MethodPost, "/"+habit.ID+"/complete", nil)
	rec2 := do(t, h, http.MethodPost, "/"+habit.ID+"/complete", nil)
	if rec2.Code != http.StatusConflict {
		t.Errorf("expected 409 on duplicate completion, got %d", rec2.Code)
	}
}

func TestHabit_Complete_withDate(t *testing.T) {
	h := newTestHandler(t)
	rec := do(t, h, http.MethodPost, "/", map[string]any{"title": "Walk"})
	var habit habits.Habit
	json.NewDecoder(rec.Body).Decode(&habit)

	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	rec2 := do(t, h, http.MethodPost, "/"+habit.ID+"/complete", map[string]any{"date": yesterday})
	if rec2.Code != http.StatusCreated {
		t.Fatalf("expected 201 for backdated completion, got %d: %s", rec2.Code, rec2.Body)
	}
}

func TestHabit_Uncomplete(t *testing.T) {
	h := newTestHandler(t)
	rec := do(t, h, http.MethodPost, "/", map[string]any{"title": "Stretch"})
	var habit habits.Habit
	json.NewDecoder(rec.Body).Decode(&habit)

	rec2 := do(t, h, http.MethodPost, "/"+habit.ID+"/complete", nil)
	var completion habits.Completion
	json.NewDecoder(rec2.Body).Decode(&completion)

	rec3 := do(t, h, http.MethodDelete, "/"+habit.ID+"/complete/"+completion.ID, nil)
	if rec3.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec3.Code, rec3.Body)
	}
}

func TestHabit_Update(t *testing.T) {
	h := newTestHandler(t)
	rec := do(t, h, http.MethodPost, "/", map[string]any{"title": "Old"})
	var habit habits.Habit
	json.NewDecoder(rec.Body).Decode(&habit)

	rec2 := do(t, h, http.MethodPatch, "/"+habit.ID, map[string]any{"title": "New"})
	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec2.Code, rec2.Body)
	}
	var updated habits.Habit
	json.NewDecoder(rec2.Body).Decode(&updated)
	if updated.Title != "New" {
		t.Errorf("title not updated: %q", updated.Title)
	}
}

func TestHabit_Delete(t *testing.T) {
	h := newTestHandler(t)
	rec := do(t, h, http.MethodPost, "/", map[string]any{"title": "Bye"})
	var habit habits.Habit
	json.NewDecoder(rec.Body).Decode(&habit)

	rec2 := do(t, h, http.MethodDelete, "/"+habit.ID, nil)
	if rec2.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec2.Code)
	}
	rec3 := do(t, h, http.MethodGet, "/"+habit.ID, nil)
	if rec3.Code != http.StatusNotFound {
		t.Errorf("expected 404 after delete, got %d", rec3.Code)
	}
}
