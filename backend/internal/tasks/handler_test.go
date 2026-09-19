package tasks_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"homeqol/internal/tasks"
)

// newTestHandler builds a fully wired Handler backed by an in-memory DB.
// This is an integration test — it exercises handler + service + repository together.
func newTestHandler(t *testing.T) http.Handler {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))
	database := openTestDB(t, logger)
	repo := tasks.NewRepository(database)
	svc := tasks.NewService(repo, logger)
	return tasks.NewRouter(svc)
}

func doRequest(t *testing.T, handler http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode request body: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

// ── POST / (Create) ───────────────────────────────────────────────────────────

func TestHandler_Create_201(t *testing.T) {
	h := newTestHandler(t)

	rec := doRequest(t, h, http.MethodPost, "/", map[string]any{
		"title":    "Implement CI pipeline",
		"priority": 1,
	})

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var task tasks.Task
	if err := json.NewDecoder(rec.Body).Decode(&task); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if task.ID == "" {
		t.Error("expected non-empty ID in response")
	}
	if task.Title != "Implement CI pipeline" {
		t.Errorf("unexpected title: %q", task.Title)
	}
	if task.Status != tasks.StatusTodo {
		t.Errorf("unexpected status: %q", task.Status)
	}
}

func TestHandler_Create_400_missingTitle(t *testing.T) {
	h := newTestHandler(t)
	rec := doRequest(t, h, http.MethodPost, "/", map[string]any{"priority": 1})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestHandler_Create_400_badJSON(t *testing.T) {
	h := newTestHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{bad json`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

// ── GET /{id} ─────────────────────────────────────────────────────────────────

func TestHandler_Get_200(t *testing.T) {
	h := newTestHandler(t)

	// Create a task first.
	createRec := doRequest(t, h, http.MethodPost, "/", map[string]any{"title": "Get me"})
	var created tasks.Task
	json.NewDecoder(createRec.Body).Decode(&created)

	rec := doRequest(t, h, http.MethodGet, "/"+created.ID, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var fetched tasks.Task
	json.NewDecoder(rec.Body).Decode(&fetched)
	if fetched.ID != created.ID {
		t.Errorf("ID mismatch: got %q, want %q", fetched.ID, created.ID)
	}
}

func TestHandler_Get_404(t *testing.T) {
	h := newTestHandler(t)
	rec := doRequest(t, h, http.MethodGet, "/does-not-exist", nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

// ── GET / (List) ──────────────────────────────────────────────────────────────

func TestHandler_List_200_emptySlice(t *testing.T) {
	h := newTestHandler(t)
	rec := doRequest(t, h, http.MethodGet, "/", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)

	// tasks key must be present and be an array (not null).
	tasksVal, ok := resp["tasks"]
	if !ok {
		t.Fatal("response missing 'tasks' key")
	}
	if tasksVal == nil {
		t.Error("'tasks' should be [] not null")
	}
}

func TestHandler_List_filtersByStatus(t *testing.T) {
	h := newTestHandler(t)

	// Create a task and complete it.
	createRec := doRequest(t, h, http.MethodPost, "/", map[string]any{"title": "Done task"})
	var created tasks.Task
	json.NewDecoder(createRec.Body).Decode(&created)
	doRequest(t, h, http.MethodPost, "/"+created.ID+"/complete", nil)

	// List only todo tasks — should not include the completed one.
	rec := doRequest(t, h, http.MethodGet, "/?status=todo", nil)
	var resp struct {
		Tasks []tasks.Task `json:"tasks"`
	}
	json.NewDecoder(rec.Body).Decode(&resp)
	for _, task := range resp.Tasks {
		if task.Status == tasks.StatusDone {
			t.Errorf("found done task in todo list: %s", task.ID)
		}
	}
}

// ── PATCH /{id} ───────────────────────────────────────────────────────────────

func TestHandler_Update_200(t *testing.T) {
	h := newTestHandler(t)

	createRec := doRequest(t, h, http.MethodPost, "/", map[string]any{"title": "Old title"})
	var created tasks.Task
	json.NewDecoder(createRec.Body).Decode(&created)

	rec := doRequest(t, h, http.MethodPatch, "/"+created.ID, map[string]any{
		"title": "New title",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var updated tasks.Task
	json.NewDecoder(rec.Body).Decode(&updated)
	if updated.Title != "New title" {
		t.Errorf("title not updated: %q", updated.Title)
	}
}

func TestHandler_Update_404(t *testing.T) {
	h := newTestHandler(t)
	rec := doRequest(t, h, http.MethodPatch, "/ghost", map[string]any{"title": "x"})
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestHandler_Update_400_invalidTransition(t *testing.T) {
	h := newTestHandler(t)

	createRec := doRequest(t, h, http.MethodPost, "/", map[string]any{"title": "Task"})
	var created tasks.Task
	json.NewDecoder(createRec.Body).Decode(&created)
	doRequest(t, h, http.MethodPost, "/"+created.ID+"/complete", nil)

	// done → in_progress is forbidden.
	rec := doRequest(t, h, http.MethodPatch, "/"+created.ID, map[string]any{
		"status": "in_progress",
	})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid transition, got %d", rec.Code)
	}
}

// ── POST /{id}/complete ───────────────────────────────────────────────────────

func TestHandler_Complete_200(t *testing.T) {
	h := newTestHandler(t)

	createRec := doRequest(t, h, http.MethodPost, "/", map[string]any{"title": "Finish me"})
	var created tasks.Task
	json.NewDecoder(createRec.Body).Decode(&created)

	rec := doRequest(t, h, http.MethodPost, "/"+created.ID+"/complete", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var completed tasks.Task
	json.NewDecoder(rec.Body).Decode(&completed)
	if completed.Status != tasks.StatusDone {
		t.Errorf("expected done, got %q", completed.Status)
	}
}

// ── DELETE /{id} ──────────────────────────────────────────────────────────────

func TestHandler_Delete_204(t *testing.T) {
	h := newTestHandler(t)

	createRec := doRequest(t, h, http.MethodPost, "/", map[string]any{"title": "Delete me"})
	var created tasks.Task
	json.NewDecoder(createRec.Body).Decode(&created)

	rec := doRequest(t, h, http.MethodDelete, "/"+created.ID, nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}

	// Confirm it's gone.
	getRec := doRequest(t, h, http.MethodGet, "/"+created.ID, nil)
	if getRec.Code != http.StatusNotFound {
		t.Errorf("expected 404 after delete, got %d", getRec.Code)
	}
}

func TestHandler_Delete_404(t *testing.T) {
	h := newTestHandler(t)
	rec := doRequest(t, h, http.MethodDelete, "/ghost", nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}
