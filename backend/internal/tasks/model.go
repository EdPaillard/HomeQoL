// Package tasks implements the task management domain:
// CRUD for tasks, tag assignment, filtering, and soft-delete.
//
// Layer responsibilities:
//
//	model.go      — pure Go types, constants, validation. Zero external deps.
//	repository.go — SQL queries. Knows about *db.DB, knows nothing about HTTP.
//	service.go    — business logic. Calls repository, enforces rules.
//	handler.go    — HTTP layer. Decodes requests, calls service, encodes responses.
//	router.go     — mounts the handler methods onto Chi routes.
package tasks

import (
	"errors"
	"strings"
	"time"
)

// ── Status ───────────────────────────────────────────────────────────────────

type Status string

const (
	StatusTodo       Status = "todo"
	StatusInProgress Status = "in_progress"
	StatusDone       Status = "done"
	StatusCancelled  Status = "cancelled"
)

func (s Status) Valid() bool {
	switch s {
	case StatusTodo, StatusInProgress, StatusDone, StatusCancelled:
		return true
	}
	return false
}

// ── Priority ─────────────────────────────────────────────────────────────────

type Priority int

const (
	PriorityHigh   Priority = 1
	PriorityMedium Priority = 2
	PriorityLow    Priority = 3
)

func (p Priority) Valid() bool {
	return p >= PriorityHigh && p <= PriorityLow
}

func (p Priority) String() string {
	switch p {
	case PriorityHigh:
		return "high"
	case PriorityMedium:
		return "medium"
	case PriorityLow:
		return "low"
	}
	return "unknown"
}

// ── Tag ──────────────────────────────────────────────────────────────────────

// Tag is a free-form label that can be attached to one or many tasks.
type Tag struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Color     string    `json:"color"`
	CreatedAt time.Time `json:"created_at"`
}

// ── Task ─────────────────────────────────────────────────────────────────────

// Task is the core domain entity. It is what flows between all layers.
// JSON tags are camelCase for API consumers (React, Flutter).
type Task struct {
	ID          string     `json:"id"`
	GoalID      *string    `json:"goal_id,omitempty"`  // nil if not linked to a goal
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Status      Status     `json:"status"`
	Priority    Priority   `json:"priority"`
	DueDate     *string    `json:"due_date,omitempty"` // "YYYY-MM-DD" or nil
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	Tags        []Tag      `json:"tags"`               // always present, empty slice if none
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// IsOverdue returns true if the task has a due date in the past and is not done.
func (t *Task) IsOverdue() bool {
	if t.DueDate == nil {
		return false
	}
	if t.Status == StatusDone || t.Status == StatusCancelled {
		return false
	}
	due, err := time.Parse("2006-01-02", *t.DueDate)
	if err != nil {
		return false
	}
	return due.Before(time.Now().Truncate(24 * time.Hour))
}

// ── Request / response DTOs ───────────────────────────────────────────────────
// These are the shapes the HTTP handler accepts. Keeping them separate from
// the domain Task means the API contract can evolve independently of the model.

// CreateRequest is the body for POST /api/v1/tasks.
type CreateRequest struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Priority    Priority `json:"priority"`
	GoalID      *string  `json:"goal_id"`
	DueDate     *string  `json:"due_date"` // "YYYY-MM-DD"
	TagIDs      []string `json:"tag_ids"`
}

func (r *CreateRequest) Validate() error {
	r.Title = strings.TrimSpace(r.Title)
	if r.Title == "" {
		return errors.New("title is required")
	}
	if len(r.Title) > 255 {
		return errors.New("title must be 255 characters or fewer")
	}
	if r.Priority == 0 {
		r.Priority = PriorityMedium // default
	}
	if !r.Priority.Valid() {
		return errors.New("priority must be 1 (high), 2 (medium), or 3 (low)")
	}
	if r.DueDate != nil {
		if _, err := time.Parse("2006-01-02", *r.DueDate); err != nil {
			return errors.New("due_date must be in YYYY-MM-DD format")
		}
	}
	return nil
}

// UpdateRequest is the body for PATCH /api/v1/tasks/{id}.
// All fields are pointers so we can distinguish "not provided" from zero value —
// only non-nil fields are applied (partial update / JSON Merge Patch style).
type UpdateRequest struct {
	Title       *string   `json:"title"`
	Description *string   `json:"description"`
	Status      *Status   `json:"status"`
	Priority    *Priority `json:"priority"`
	GoalID      *string   `json:"goal_id"`
	DueDate     *string   `json:"due_date"`
	TagIDs      []string  `json:"tag_ids"` // nil = don't touch tags; [] = remove all tags
}

func (r *UpdateRequest) Validate() error {
	if r.Title != nil {
		*r.Title = strings.TrimSpace(*r.Title)
		if *r.Title == "" {
			return errors.New("title cannot be empty")
		}
		if len(*r.Title) > 255 {
			return errors.New("title must be 255 characters or fewer")
		}
	}
	if r.Status != nil && !r.Status.Valid() {
		return errors.New("status must be one of: todo, in_progress, done, cancelled")
	}
	if r.Priority != nil && !r.Priority.Valid() {
		return errors.New("priority must be 1 (high), 2 (medium), or 3 (low)")
	}
	if r.DueDate != nil && *r.DueDate != "" {
		if _, err := time.Parse("2006-01-02", *r.DueDate); err != nil {
			return errors.New("due_date must be in YYYY-MM-DD format")
		}
	}
	return nil
}

// ── Filters ──────────────────────────────────────────────────────────────────

// ListFilter is built from query parameters and passed to the repository.
type ListFilter struct {
	Status   *Status  // ?status=todo
	Priority *Priority // ?priority=1
	GoalID   *string  // ?goal_id=uuid
	TagID    *string  // ?tag_id=uuid
	DueToday bool     // ?due_today=true
	Overdue  bool     // ?overdue=true
	Search   string   // ?q=keyword (matched against title)

	// Pagination
	Limit  int // default 50, max 200
	Offset int // for cursor-style paging
}

// ── Sentinel errors ──────────────────────────────────────────────────────────
// Using typed sentinel errors lets the service and handler distinguish
// "not found" from "database error" without string matching.

var (
	ErrNotFound   = errors.New("task not found")
	ErrTagNotFound = errors.New("one or more tags not found")
)
