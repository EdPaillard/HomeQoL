package habits_test

import (
	"log/slog"
	"os"
	"testing"

	"homeqol/internal/tasks"
)

// fakeRepo is an in-memory implementation of the repository interface used
// by the service. This lets us test service logic without a real database.
//
// In a larger project you'd define a Repository interface and inject it;
// for this project, a concrete fake is simpler and sufficient.
type fakeRepo struct {
	store map[string]*tasks.Task
	tags  map[string]tasks.Tag
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		store: make(map[string]*tasks.Task),
		tags:  make(map[string]tasks.Tag),
	}
}

// We test the service indirectly through the service methods, using a real
// in-memory SQLite database rather than mocking the repository.
// This is the integration-style approach: test the service + repository together
// with a real (but temporary) DB.

func newTestService(t *testing.T) *tasks.Service {
	t.Helper()

	// Import the db package to open an in-memory SQLite database.
	// The ":memory:" path creates a temporary database that is discarded
	// when the connection closes — perfect for tests.
	//
	// Note: we skip this if go-sqlite3 is not available (e.g. no CGO).
	// In CI, CGO_ENABLED=1 is set so this always runs.
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelError, // silence info logs during tests
	}))

	database := openTestDB(t, logger)
	repo := tasks.NewRepository(database)
	return tasks.NewService(repo, logger)
}

// ── Service tests (require DB) ────────────────────────────────────────────────

func TestService_CreateAndGet(t *testing.T) {
	svc := newTestService(t)

	req := tasks.CreateRequest{
		Title:    "Write the handler tests",
		Priority: tasks.PriorityHigh,
	}

	created, err := svc.Create(req)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if created.ID == "" {
		t.Error("expected non-empty ID")
	}
	if created.Title != "Write the handler tests" {
		t.Errorf("unexpected title: %q", created.Title)
	}
	if created.Status != tasks.StatusTodo {
		t.Errorf("expected todo status, got %q", created.Status)
	}

	// Fetch it back.
	fetched, err := svc.Get(created.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if fetched.ID != created.ID {
		t.Errorf("ID mismatch: got %q, want %q", fetched.ID, created.ID)
	}
}

func TestService_Create_validationError(t *testing.T) {
	svc := newTestService(t)
	_, err := svc.Create(tasks.CreateRequest{Title: ""})
	if err == nil {
		t.Error("expected validation error for empty title")
	}
}

func TestService_Get_notFound(t *testing.T) {
	svc := newTestService(t)
	_, err := svc.Get("non-existent-id")
	if err == nil {
		t.Error("expected ErrNotFound")
	}
}

func TestService_List_filtersbyStatus(t *testing.T) {
	svc := newTestService(t)

	// Create two tasks with different priorities/statuses.
	svc.Create(tasks.CreateRequest{Title: "Task A", Priority: tasks.PriorityHigh})
	taskB, _ := svc.Create(tasks.CreateRequest{Title: "Task B", Priority: tasks.PriorityLow})

	// Complete task B.
	svc.Complete(taskB.ID)

	todo := tasks.StatusTodo
	listed, err := svc.List(tasks.ListFilter{Status: &todo})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	for _, t2 := range listed {
		if t2.Status != tasks.StatusTodo {
			t.Errorf("expected todo task, got %q", t2.Status)
		}
	}
}

func TestService_Update_partialUpdate(t *testing.T) {
	svc := newTestService(t)

	created, _ := svc.Create(tasks.CreateRequest{
		Title:    "Original title",
		Priority: tasks.PriorityLow,
	})

	newTitle := "Updated title"
	updated, err := svc.Update(created.ID, tasks.UpdateRequest{Title: &newTitle})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if updated.Title != "Updated title" {
		t.Errorf("title not updated: %q", updated.Title)
	}
	// Priority should be unchanged.
	if updated.Priority != tasks.PriorityLow {
		t.Errorf("priority changed unexpectedly: %d", updated.Priority)
	}
}

func TestService_Complete_setsCompletedAt(t *testing.T) {
	svc := newTestService(t)

	created, _ := svc.Create(tasks.CreateRequest{
		Title: "Finish the API",
	})

	completed, err := svc.Complete(created.ID)
	if err != nil {
		t.Fatalf("Complete failed: %v", err)
	}
	if completed.Status != tasks.StatusDone {
		t.Errorf("expected done, got %q", completed.Status)
	}
	if completed.CompletedAt == nil {
		t.Error("expected completed_at to be set")
	}
}

func TestService_StatusTransition_invalidTransitionRejected(t *testing.T) {
	svc := newTestService(t)

	created, _ := svc.Create(tasks.CreateRequest{Title: "Task"})
	svc.Complete(created.ID)

	// done → in_progress is not allowed per our state machine.
	inProgress := tasks.StatusInProgress
	_, err := svc.Update(created.ID, tasks.UpdateRequest{Status: &inProgress})
	if err == nil {
		t.Error("expected error for invalid status transition done→in_progress")
	}
}

func TestService_StatusTransition_doneCanReopenToTodo(t *testing.T) {
	svc := newTestService(t)

	created, _ := svc.Create(tasks.CreateRequest{Title: "Task"})
	svc.Complete(created.ID)

	// done → todo is allowed (re-open).
	todo := tasks.StatusTodo
	reopened, err := svc.Update(created.ID, tasks.UpdateRequest{Status: &todo})
	if err != nil {
		t.Fatalf("expected re-open to succeed, got: %v", err)
	}
	if reopened.Status != tasks.StatusTodo {
		t.Errorf("expected todo, got %q", reopened.Status)
	}
	if reopened.CompletedAt != nil {
		t.Error("completed_at should be cleared after re-open")
	}
}

func TestService_Delete_softDelete(t *testing.T) {
	svc := newTestService(t)

	created, _ := svc.Create(tasks.CreateRequest{Title: "To be deleted"})

	if err := svc.Delete(created.ID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Should no longer be findable.
	_, err := svc.Get(created.ID)
	if err == nil {
		t.Error("expected ErrNotFound after delete")
	}
}

func TestService_Delete_notFound(t *testing.T) {
	svc := newTestService(t)
	err := svc.Delete("ghost-id")
	if err == nil {
		t.Error("expected ErrNotFound for non-existent task")
	}
}
