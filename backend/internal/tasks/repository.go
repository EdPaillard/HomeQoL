package tasks

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"homeqol/internal/db"
)

// Repository handles all database operations for the tasks domain.
// It is the only layer that knows SQL. The service calls it; it never calls
// the service.
type Repository struct {
	db *db.DB
}

// NewRepository creates a Repository backed by the given database.
func NewRepository(database *db.DB) *Repository {
	return &Repository{db: database}
}

// ── Read operations ───────────────────────────────────────────────────────────

// GetByID fetches a single task with its tags.
// Returns ErrNotFound if the task doesn't exist or was soft-deleted.
func (r *Repository) GetByID(id string) (*Task, error) {
	const q = `
		SELECT
			id, goal_id, title, description, status, priority,
			due_date, completed_at, created_at, updated_at
		FROM tasks
		WHERE id = ? AND deleted_at IS NULL`

	task, err := r.scanTask(r.db.QueryRow(q, id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get task %s: %w", id, err)
	}

	if err := r.loadTags(task); err != nil {
		return nil, err
	}
	return task, nil
}

// List returns tasks matching the filter, ordered by priority ASC then due_date ASC.
func (r *Repository) List(f ListFilter) ([]Task, error) {
	// Build the WHERE clause dynamically based on which filters are set.
	// Using a slice of conditions + args keeps it readable and injection-safe.
	conditions := []string{"t.deleted_at IS NULL"}
	args := []any{}

	if f.Status != nil {
		conditions = append(conditions, "t.status = ?")
		args = append(args, string(*f.Status))
	}
	if f.Priority != nil {
		conditions = append(conditions, "t.priority = ?")
		args = append(args, int(*f.Priority))
	}
	if f.GoalID != nil {
		conditions = append(conditions, "t.goal_id = ?")
		args = append(args, *f.GoalID)
	}
	if f.Search != "" {
		conditions = append(conditions, "t.title LIKE ?")
		args = append(args, "%"+f.Search+"%")
	}
	if f.DueToday {
		today := time.Now().Format("2006-01-02")
		conditions = append(conditions, "t.due_date = ?")
		args = append(args, today)
	}
	if f.Overdue {
		today := time.Now().Format("2006-01-02")
		conditions = append(conditions, "t.due_date < ? AND t.status NOT IN ('done','cancelled')")
		args = append(args, today)
	}
	if f.TagID != nil {
		// Tasks that have at least one matching tag via the join table.
		conditions = append(conditions, `EXISTS (
			SELECT 1 FROM task_tags tt WHERE tt.task_id = t.id AND tt.tag_id = ?
		)`)
		args = append(args, *f.TagID)
	}

	limit := f.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	offset := max(f.Offset, 0)

	query := fmt.Sprintf(`
		SELECT
			t.id, t.goal_id, t.title, t.description, t.status, t.priority,
			t.due_date, t.completed_at, t.created_at, t.updated_at
		FROM tasks t
		WHERE %s
		ORDER BY t.priority ASC, t.due_date ASC NULLS LAST, t.created_at ASC
		LIMIT ? OFFSET ?`,
		strings.Join(conditions, " AND "),
	)
	args = append(args, limit, offset)

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	defer rows.Close()

	tasks := []Task{} // never return nil — JSON encodes as [] not null
	for rows.Next() {
		task, err := r.scanTaskRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan task: %w", err)
		}
		tasks = append(tasks, *task)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tasks: %w", err)
	}

	// Load tags for all tasks in one query (N+1 prevention).
	if err := r.loadTagsForMany(tasks); err != nil {
		return nil, err
	}
	return tasks, nil
}

// ── Write operations ──────────────────────────────────────────────────────────

// Create inserts a new task and attaches any provided tag IDs.
// The caller must supply a pre-generated UUID for the ID.
func (r *Repository) Create(id string, req CreateRequest) (*Task, error) {
	const q = `
		INSERT INTO tasks (id, goal_id, title, description, priority, due_date)
		VALUES (?, ?, ?, ?, ?, ?)
	`
	_, err := r.db.Exec(q,
		id,
		req.GoalID,
		req.Title,
		req.Description,
		int(req.Priority),
		req.DueDate,
	)
	if err != nil {
		return nil, fmt.Errorf("insert task: %w", err)
	}

	if len(req.TagIDs) > 0 {
		if err := r.setTags(id, req.TagIDs); err != nil {
			return nil, err
		}
	}

	return r.GetByID(id)
}

// Update applies a partial update to a task.
// Only non-nil fields in UpdateRequest are written.
func (r *Repository) Update(id string, req UpdateRequest) (*Task, error) {
	// Read the current task first to apply partial updates.
	existing, err := r.GetByID(id)
	if err != nil {
		return nil, err // propagates ErrNotFound
	}

	// Build SET clause from non-nil fields only.
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
	if req.Priority != nil {
		setClauses = append(setClauses, "priority = ?")
		args = append(args, int(*req.Priority))
	}
	if req.GoalID != nil {
		setClauses = append(setClauses, "goal_id = ?")
		args = append(args, *req.GoalID)
	}
	if req.DueDate != nil {
		if *req.DueDate == "" {
			setClauses = append(setClauses, "due_date = NULL")
		} else {
			setClauses = append(setClauses, "due_date = ?")
			args = append(args, *req.DueDate)
		}
	}
	if req.Status != nil {
		setClauses = append(setClauses, "status = ?")
		args = append(args, string(*req.Status))

		// Set completed_at when transitioning to done; clear it otherwise.
		if *req.Status == StatusDone && existing.Status != StatusDone {
			now := time.Now().UTC().Format(time.RFC3339)
			setClauses = append(setClauses, "completed_at = ?")
			args = append(args, now)
		} else if *req.Status != StatusDone {
			setClauses = append(setClauses, "completed_at = NULL")
		}
	}

	if len(setClauses) > 0 {
		query := fmt.Sprintf(
			"UPDATE tasks SET %s WHERE id = ? AND deleted_at IS NULL",
			strings.Join(setClauses, ", "),
		)
		args = append(args, id)
		if _, err := r.db.Exec(query, args...); err != nil {
			return nil, fmt.Errorf("update task %s: %w", id, err)
		}
	}

	// Tag IDs present (even empty slice) means "replace tags".
	// nil TagIDs means "don't touch tags".
	if req.TagIDs != nil {
		if err := r.setTags(id, req.TagIDs); err != nil {
			return nil, err
		}
	}

	return r.GetByID(id)
}

// Delete soft-deletes a task by setting deleted_at.
// Returns ErrNotFound if the task doesn't exist or is already deleted.
func (r *Repository) Delete(id string) error {
	const q = `
		UPDATE tasks
		SET deleted_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
		WHERE id = ? AND deleted_at IS NULL
	`
	result, err := r.db.Exec(q, id)
	if err != nil {
		return fmt.Errorf("delete task %s: %w", id, err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// ── Tag helpers ───────────────────────────────────────────────────────────────

// setTags replaces all tags on a task with the given tag IDs.
// It verifies that all provided tag IDs exist before making changes,
// so you never silently ignore an invalid tag ID.
func (r *Repository) setTags(taskID string, tagIDs []string) error {
	if len(tagIDs) > 0 {
		// Verify all tag IDs exist.
		placeholders := strings.Repeat("?,", len(tagIDs))
		placeholders = placeholders[:len(placeholders)-1]
		var count int
		args := make([]any, len(tagIDs))
		for i, id := range tagIDs {
			args[i] = id
		}
		err := r.db.QueryRow(
			fmt.Sprintf("SELECT COUNT(*) FROM tags WHERE id IN (%s)", placeholders),
			args...,
		).Scan(&count)
		if err != nil {
			return fmt.Errorf("verify tags: %w", err)
		}
		if count != len(tagIDs) {
			return ErrTagNotFound
		}
	}

	// Replace in a transaction: delete existing, insert new.
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tag transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM task_tags WHERE task_id = ?", taskID); err != nil {
		return fmt.Errorf("clear task tags: %w", err)
	}

	for _, tagID := range tagIDs {
		if _, err := tx.Exec(
			"INSERT INTO task_tags (task_id, tag_id) VALUES (?, ?)",
			taskID, tagID,
		); err != nil {
			return fmt.Errorf("insert task tag: %w", err)
		}
	}

	return tx.Commit()
}

// loadTags fetches the tags for a single task (used after GetByID).
func (r *Repository) loadTags(task *Task) error {
	tasks := []Task{*task}
	if err := r.loadTagsForMany(tasks); err != nil {
		return err
	}
	task.Tags = tasks[0].Tags
	return nil
}

// loadTagsForMany fetches tags for a slice of tasks in a single SQL query,
// avoiding the N+1 problem. Each task's Tags slice is populated in-place.
func (r *Repository) loadTagsForMany(tasks []Task) error {
	if len(tasks) == 0 {
		return nil
	}

	// Build an index for fast lookup.
	index := make(map[string]*Task, len(tasks))
	ids := make([]any, len(tasks))
	for i := range tasks {
		tasks[i].Tags = []Tag{} // ensure empty slice, not nil
		index[tasks[i].ID] = &tasks[i]
		ids[i] = tasks[i].ID
	}

	placeholders := strings.Repeat("?,", len(ids))
	placeholders = placeholders[:len(placeholders)-1]

	rows, err := r.db.Query(fmt.Sprintf(`
		SELECT tt.task_id, t.id, t.name, t.color, t.created_at
		FROM task_tags tt
		JOIN tags t ON t.id = tt.tag_id
		WHERE tt.task_id IN (%s)
		ORDER BY t.name ASC
	`, placeholders), ids...)
	if err != nil {
		return fmt.Errorf("load tags: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var taskID string
		var tag Tag
		var createdAt string
		if err := rows.Scan(&taskID, &tag.ID, &tag.Name, &tag.Color, &createdAt); err != nil {
			return fmt.Errorf("scan tag: %w", err)
		}
		tag.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		if t, ok := index[taskID]; ok {
			t.Tags = append(t.Tags, tag)
		}
	}
	return rows.Err()
}

// ── Scan helpers ──────────────────────────────────────────────────────────────
// Centralised scanning prevents drift between GetByID and List.

type scanner interface {
	Scan(dest ...any) error
}

func (r *Repository) scanTask(row scanner) (*Task, error) {
	return r.scanTaskRow(row)
}

func (r *Repository) scanTaskRow(row scanner) (*Task, error) {
	var t Task
	var goalID sql.NullString
	var dueDate sql.NullString
	var completedAt sql.NullString
	var createdAt, updatedAt string
	var status string
	var priority int

	err := row.Scan(
		&t.ID,
		&goalID,
		&t.Title,
		&t.Description,
		&status,
		&priority,
		&dueDate,
		&completedAt,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return nil, err
	}

	t.Status = Status(status)
	t.Priority = Priority(priority)

	if goalID.Valid {
		t.GoalID = &goalID.String
	}
	if dueDate.Valid {
		t.DueDate = &dueDate.String
	}
	if completedAt.Valid {
		parsed, _ := time.Parse(time.RFC3339, completedAt.String)
		t.CompletedAt = &parsed
	}

	t.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	t.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	t.Tags = []Tag{}

	return &t, nil
}
