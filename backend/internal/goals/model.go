// Package goals implements the goals domain: OKR-style objectives with
// optional milestone (child goal) support and linkage to tasks.
package goals

import (
	"errors"
	"strings"
	"time"
)

// ── Status ────────────────────────────────────────────────────────────────────

type Status string

const (
	StatusActive    Status = "active"
	StatusCompleted Status = "completed"
	StatusAbandoned Status = "abandoned"
)

func (s Status) Valid() bool {
	switch s {
	case StatusActive, StatusCompleted, StatusAbandoned:
		return true
	}
	return false
}

// ── Goal ──────────────────────────────────────────────────────────────────────

// Goal is either a top-level objective or a milestone (if ParentID is set).
type Goal struct {
	ID          string     `json:"id"`
	ParentID    *string    `json:"parent_id,omitempty"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Status      Status     `json:"status"`
	TargetDate  *string    `json:"target_date,omitempty"` // "YYYY-MM-DD"
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	// Computed fields populated by the service.
	Milestones  []Goal    `json:"milestones,omitempty"`
	TaskCount   int       `json:"task_count"`
	DoneCount   int       `json:"done_count"`
	ProgressPct float64   `json:"progress_pct"` // done/total * 100
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ── DTOs ──────────────────────────────────────────────────────────────────────

type CreateRequest struct {
	ParentID    *string `json:"parent_id"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	TargetDate  *string `json:"target_date"`
}

func (r *CreateRequest) Validate() error {
	r.Title = strings.TrimSpace(r.Title)
	if r.Title == "" {
		return errors.New("title is required")
	}
	if len(r.Title) > 255 {
		return errors.New("title must be 255 characters or fewer")
	}
	if r.TargetDate != nil {
		if _, err := time.Parse("2006-01-02", *r.TargetDate); err != nil {
			return errors.New("target_date must be YYYY-MM-DD")
		}
	}
	return nil
}

type UpdateRequest struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
	Status      *Status `json:"status"`
	TargetDate  *string `json:"target_date"`
}

func (r *UpdateRequest) Validate() error {
	if r.Title != nil {
		*r.Title = strings.TrimSpace(*r.Title)
		if *r.Title == "" {
			return errors.New("title cannot be empty")
		}
	}
	if r.Status != nil && !r.Status.Valid() {
		return errors.New("status must be active, completed, or abandoned")
	}
	if r.TargetDate != nil && *r.TargetDate != "" {
		if _, err := time.Parse("2006-01-02", *r.TargetDate); err != nil {
			return errors.New("target_date must be YYYY-MM-DD")
		}
	}
	return nil
}

// ── Errors ────────────────────────────────────────────────────────────────────

var (
	ErrNotFound    = errors.New("goal not found")
	ErrHasChildren = errors.New("cannot delete a goal that has milestones")
)
