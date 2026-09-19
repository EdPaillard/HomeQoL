package tasks_test

import (
	"testing"
	"time"

	"homeqol/internal/tasks"
)

// ── CreateRequest.Validate ────────────────────────────────────────────────────

func TestCreateRequest_Validate_requiresTitle(t *testing.T) {
	req := tasks.CreateRequest{Priority: tasks.PriorityMedium}
	if err := req.Validate(); err == nil {
		t.Error("expected error for empty title")
	}
}

func TestCreateRequest_Validate_trimsTitleWhitespace(t *testing.T) {
	req := tasks.CreateRequest{Title: "  Buy milk  ", Priority: tasks.PriorityHigh}
	if err := req.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.Title != "Buy milk" {
		t.Errorf("expected trimmed title, got %q", req.Title)
	}
}

func TestCreateRequest_Validate_defaultsPriorityToMedium(t *testing.T) {
	req := tasks.CreateRequest{Title: "Task"}
	if err := req.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.Priority != tasks.PriorityMedium {
		t.Errorf("expected PriorityMedium, got %d", req.Priority)
	}
}

func TestCreateRequest_Validate_rejectsBadDueDate(t *testing.T) {
	bad := "31/12/2025" // wrong format
	req := tasks.CreateRequest{Title: "Task", DueDate: &bad}
	if err := req.Validate(); err == nil {
		t.Error("expected error for bad due_date format")
	}
}

func TestCreateRequest_Validate_acceptsGoodDueDate(t *testing.T) {
	good := "2025-12-31"
	req := tasks.CreateRequest{Title: "Task", DueDate: &good}
	if err := req.Validate(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// ── UpdateRequest.Validate ────────────────────────────────────────────────────

func TestUpdateRequest_Validate_rejectsEmptyTitle(t *testing.T) {
	empty := ""
	req := tasks.UpdateRequest{Title: &empty}
	if err := req.Validate(); err == nil {
		t.Error("expected error for empty title")
	}
}

func TestUpdateRequest_Validate_rejectsBadStatus(t *testing.T) {
	bad := tasks.Status("flying")
	req := tasks.UpdateRequest{Status: &bad}
	if err := req.Validate(); err == nil {
		t.Error("expected error for bad status")
	}
}

func TestUpdateRequest_Validate_nilFieldsAreIgnored(t *testing.T) {
	req := tasks.UpdateRequest{} // everything nil
	if err := req.Validate(); err != nil {
		t.Errorf("empty UpdateRequest should be valid, got: %v", err)
	}
}

// ── Task.IsOverdue ────────────────────────────────────────────────────────────

func TestTask_IsOverdue_noDueDateIsNotOverdue(t *testing.T) {
	task := tasks.Task{Status: tasks.StatusTodo}
	if task.IsOverdue() {
		t.Error("task without due date should not be overdue")
	}
}

func TestTask_IsOverdue_pastDateAndTodoIsOverdue(t *testing.T) {
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	task := tasks.Task{Status: tasks.StatusTodo, DueDate: &yesterday}
	if !task.IsOverdue() {
		t.Error("past due date with todo status should be overdue")
	}
}

func TestTask_IsOverdue_doneTaskIsNeverOverdue(t *testing.T) {
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	task := tasks.Task{Status: tasks.StatusDone, DueDate: &yesterday}
	if task.IsOverdue() {
		t.Error("done task should never be overdue")
	}
}

func TestTask_IsOverdue_futureDateIsNotOverdue(t *testing.T) {
	tomorrow := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	task := tasks.Task{Status: tasks.StatusTodo, DueDate: &tomorrow}
	if task.IsOverdue() {
		t.Error("future due date should not be overdue")
	}
}

// ── Priority ──────────────────────────────────────────────────────────────────

func TestPriority_Valid(t *testing.T) {
	cases := []struct {
		p     tasks.Priority
		valid bool
	}{
		{tasks.PriorityHigh, true},
		{tasks.PriorityMedium, true},
		{tasks.PriorityLow, true},
		{0, false},
		{4, false},
	}
	for _, c := range cases {
		if got := c.p.Valid(); got != c.valid {
			t.Errorf("Priority(%d).Valid() = %v, want %v", c.p, got, c.valid)
		}
	}
}
