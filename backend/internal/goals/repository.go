package goals

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"homeqol/internal/db"
)

// Repository handles all SQL for the goals domain.
type Repository struct {
	db *db.DB
}

func NewRepository(database *db.DB) *Repository {
	return &Repository{db: database}
}

// ── Read ──────────────────────────────────────────────────────────────────────

func (r *Repository) GetByID(id string) (*Goal, error) {
	const q = `
		SELECT id, parent_id, title, description, status,
		       target_date, completed_at, created_at, updated_at
		FROM goals
		WHERE id = ? AND deleted_at IS NULL`
	g, err := r.scan(r.db.QueryRow(q, id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get goal %s: %w", id, err)
	}
	return g, nil
}

// ListTopLevel returns goals without a parent, ordered by status then created_at.
func (r *Repository) ListTopLevel() ([]Goal, error) {
	const q = `
		SELECT id, parent_id, title, description, status,
		       target_date, completed_at, created_at, updated_at
		FROM goals
		WHERE parent_id IS NULL AND deleted_at IS NULL
		ORDER BY
			CASE status WHEN 'active' THEN 0 WHEN 'completed' THEN 1 ELSE 2 END,
			created_at ASC`
	return r.queryGoals(q)
}

// ListMilestones returns child goals for a given parent.
func (r *Repository) ListMilestones(parentID string) ([]Goal, error) {
	const q = `
		SELECT id, parent_id, title, description, status,
		       target_date, completed_at, created_at, updated_at
		FROM goals
		WHERE parent_id = ? AND deleted_at IS NULL
		ORDER BY created_at ASC`
	return r.queryGoals(q, parentID)
}

// TaskCounts returns the total and done task counts for a goal.
func (r *Repository) TaskCounts(goalID string) (total, done int, err error) {
	const q = `
		SELECT
			COUNT(*),
			COUNT(CASE WHEN status = 'done' THEN 1 END)
		FROM tasks
		WHERE goal_id = ? AND deleted_at IS NULL`
	err = r.db.QueryRow(q, goalID).Scan(&total, &done)
	return
}

// ChildCount returns the number of milestones (child goals).
func (r *Repository) ChildCount(goalID string) (int, error) {
	var n int
	err := r.db.QueryRow(
		`SELECT COUNT(*) FROM goals WHERE parent_id = ? AND deleted_at IS NULL`, goalID,
	).Scan(&n)
	return n, err
}

// ── Write ─────────────────────────────────────────────────────────────────────

func (r *Repository) Create(id string, req CreateRequest) (*Goal, error) {
	const q = `
		INSERT INTO goals (id, parent_id, title, description, target_date)
		VALUES (?, ?, ?, ?, ?)`
	_, err := r.db.Exec(q, id, req.ParentID, req.Title, req.Description, req.TargetDate)
	if err != nil {
		return nil, fmt.Errorf("insert goal: %w", err)
	}
	return r.GetByID(id)
}

func (r *Repository) Update(id string, req UpdateRequest) (*Goal, error) {
	setClauses := []string{}
	args := []any{}

	if req.Title != nil {
		setClauses = append(setClauses, "title = ?")
		args = append(args, *req.Title)
	}
	if req.Description != nil {
		setClauses = append(setClauses, "description = ?")
		args = append(args, *req.Description)
	}
	if req.TargetDate != nil {
		if *req.TargetDate == "" {
			setClauses = append(setClauses, "target_date = NULL")
		} else {
			setClauses = append(setClauses, "target_date = ?")
			args = append(args, *req.TargetDate)
		}
	}
	if req.Status != nil {
		setClauses = append(setClauses, "status = ?")
		args = append(args, string(*req.Status))
		if *req.Status == StatusCompleted {
			now := time.Now().UTC().Format(time.RFC3339)
			setClauses = append(setClauses, "completed_at = ?")
			args = append(args, now)
		} else {
			setClauses = append(setClauses, "completed_at = NULL")
		}
	}

	if len(setClauses) > 0 {
		query := fmt.Sprintf(
			"UPDATE goals SET %s WHERE id = ? AND deleted_at IS NULL",
			strings.Join(setClauses, ", "),
		)
		args = append(args, id)
		if _, err := r.db.Exec(query, args...); err != nil {
			return nil, fmt.Errorf("update goal: %w", err)
		}
	}
	return r.GetByID(id)
}

func (r *Repository) Delete(id string) error {
	res, err := r.db.Exec(
		`UPDATE goals SET deleted_at = strftime('%Y-%m-%dT%H:%M:%SZ','now') WHERE id = ? AND deleted_at IS NULL`,
		id,
	)
	if err != nil {
		return fmt.Errorf("delete goal: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func (r *Repository) queryGoals(q string, args ...any) ([]Goal, error) {
	rows, err := r.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("query goals: %w", err)
	}
	defer rows.Close()
	goals := []Goal{}
	for rows.Next() {
		g, err := r.scan(rows)
		if err != nil {
			return nil, err
		}
		goals = append(goals, *g)
	}
	return goals, rows.Err()
}

type scanner interface{ Scan(...any) error }

func (r *Repository) scan(row scanner) (*Goal, error) {
	var g Goal
	var parentID, targetDate, completedAt sql.NullString
	var status, createdAt, updatedAt string

	err := row.Scan(
		&g.ID, &parentID, &g.Title, &g.Description,
		&status, &targetDate, &completedAt, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}

	g.Status = Status(status)
	if parentID.Valid {
		g.ParentID = &parentID.String
	}
	if targetDate.Valid {
		g.TargetDate = &targetDate.String
	}
	if completedAt.Valid {
		t, _ := time.Parse(time.RFC3339, completedAt.String)
		g.CompletedAt = &t
	}
	g.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	g.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	g.Milestones = []Goal{}
	return &g, nil
}
