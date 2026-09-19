package habits

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"homeqol/internal/db"
)

// Repository handles all database access for the habits domain.
type Repository struct {
	db *db.DB
}

func NewRepository(database *db.DB) *Repository {
	return &Repository{db: database}
}

// ── Read ──────────────────────────────────────────────────────────────────────

func (r *Repository) GetByID(id string) (*Habit, error) {
	const q = `
		SELECT id, title, description, frequency, scheduled_days,
		       target_per_week, color, icon, is_active,
		       current_streak, longest_streak, last_completed_date,
		       created_at, updated_at
		FROM habits
		WHERE id = ? AND deleted_at IS NULL`

	h, err := r.scanHabitRow(r.db.QueryRow(q, id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get habit %s: %w", id, err)
	}
	return h, nil
}

func (r *Repository) ListActive(today string) ([]Habit, error) {
	const q = `
		SELECT id, title, description, frequency, scheduled_days,
		       target_per_week, color, icon, is_active,
		       current_streak, longest_streak, last_completed_date,
		       created_at, updated_at
		FROM habits
		WHERE is_active = 1 AND deleted_at IS NULL
		ORDER BY title ASC`

	rows, err := r.db.Query(q)
	if err != nil {
		return nil, fmt.Errorf("list habits: %w", err)
	}
	defer rows.Close()

	result := []Habit{}
	for rows.Next() {
		h, err := r.scanHabitRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan habit: %w", err)
		}
		result = append(result, *h)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Mark which habits are completed today in a single extra query.
	if err := r.markCompletedToday(result, today); err != nil {
		return nil, err
	}
	return result, nil
}

// markCompletedToday sets CompletedToday=true on any habit completed on `today`.
func (r *Repository) markCompletedToday(habits []Habit, today string) error {
	if len(habits) == 0 {
		return nil
	}

	ids := make([]any, len(habits))
	index := make(map[string]*Habit, len(habits))
	for i := range habits {
		ids[i] = habits[i].ID
		index[habits[i].ID] = &habits[i]
	}

	placeholders := strings.Repeat("?,", len(ids))
	placeholders = placeholders[:len(placeholders)-1]

	rows, err := r.db.Query(fmt.Sprintf(`
		SELECT habit_id FROM habit_completions
		WHERE completed_date = ? AND habit_id IN (%s)
	`, placeholders), append([]any{today}, ids...)...)
	if err != nil {
		return fmt.Errorf("mark completed today: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return err
		}
		if h, ok := index[id]; ok {
			h.CompletedToday = true
		}
	}
	return rows.Err()
}

// AllActiveIDs returns IDs and frequencies of all active habits.
// Used by the streak job to iterate without loading full habits.
func (r *Repository) AllActiveIDs() ([]struct {
	ID            string
	Frequency     Frequency
	TargetPerWeek int
}, error) {
	const q = `
		SELECT id, frequency, COALESCE(target_per_week, 1)
		FROM habits
		WHERE is_active = 1 AND deleted_at IS NULL`

	rows, err := r.db.Query(q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []struct {
		ID            string
		Frequency     Frequency
		TargetPerWeek int
	}
	for rows.Next() {
		var item struct {
			ID            string
			Frequency     Frequency
			TargetPerWeek int
		}
		var freq string
		if err := rows.Scan(&item.ID, &freq, &item.TargetPerWeek); err != nil {
			return nil, err
		}
		item.Frequency = Frequency(freq)
		result = append(result, item)
	}
	return result, rows.Err()
}

// CompletionDates returns all completed_date values for a habit, as strings.
// Used exclusively by the streak calculation — returns "YYYY-MM-DD" slice.
func (r *Repository) CompletionDates(habitID string) ([]string, error) {
	const q = `
		SELECT completed_date FROM habit_completions
		WHERE habit_id = ?
		ORDER BY completed_date DESC`

	rows, err := r.db.Query(q, habitID)
	if err != nil {
		return nil, fmt.Errorf("get completion dates for %s: %w", habitID, err)
	}
	defer rows.Close()

	var dates []string
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			return nil, err
		}
		dates = append(dates, d)
	}
	return dates, rows.Err()
}

// ── Write ─────────────────────────────────────────────────────────────────────

func (r *Repository) Create(id string, req CreateRequest) (*Habit, error) {
	const q = `
		INSERT INTO habits (id, title, description, frequency, scheduled_days,
		                    target_per_week, color, icon)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := r.db.Exec(q,
		id,
		req.Title,
		req.Description,
		string(req.Frequency),
		EncodeDays(req.ScheduledDays),
		req.TargetPerWeek,
		req.Color,
		req.Icon,
	)
	if err != nil {
		return nil, fmt.Errorf("insert habit: %w", err)
	}
	return r.GetByID(id)
}

func (r *Repository) Update(id string, req UpdateRequest) (*Habit, error) {
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
	if req.Frequency != nil {
		setClauses = append(setClauses, "frequency = ?")
		args = append(args, string(*req.Frequency))
	}
	if req.ScheduledDays != nil {
		setClauses = append(setClauses, "scheduled_days = ?")
		args = append(args, EncodeDays(req.ScheduledDays))
	}
	if req.TargetPerWeek != nil {
		setClauses = append(setClauses, "target_per_week = ?")
		args = append(args, *req.TargetPerWeek)
	}
	if req.Color != nil {
		setClauses = append(setClauses, "color = ?")
		args = append(args, *req.Color)
	}
	if req.Icon != nil {
		setClauses = append(setClauses, "icon = ?")
		args = append(args, *req.Icon)
	}
	if req.IsActive != nil {
		setClauses = append(setClauses, "is_active = ?")
		val := 0
		if *req.IsActive {
			val = 1
		}
		args = append(args, val)
	}

	if len(setClauses) > 0 {
		query := fmt.Sprintf(
			"UPDATE habits SET %s WHERE id = ? AND deleted_at IS NULL",
			strings.Join(setClauses, ", "),
		)
		args = append(args, id)
		if _, err := r.db.Exec(query, args...); err != nil {
			return nil, fmt.Errorf("update habit: %w", err)
		}
	}
	return r.GetByID(id)
}

func (r *Repository) Delete(id string) error {
	const q = `
		UPDATE habits SET deleted_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
		WHERE id = ? AND deleted_at IS NULL`
	res, err := r.db.Exec(q, id)
	if err != nil {
		return fmt.Errorf("delete habit: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// ── Completions ───────────────────────────────────────────────────────────────

func (r *Repository) AddCompletion(id, habitID, date, note string) (*Completion, error) {
	const q = `
		INSERT INTO habit_completions (id, habit_id, completed_date, note)
		VALUES (?, ?, ?, ?)`
	_, err := r.db.Exec(q, id, habitID, date, note)
	if err != nil {
		// SQLite unique constraint violation: already completed this day.
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return nil, ErrAlreadyDone
		}
		return nil, fmt.Errorf("add completion: %w", err)
	}
	return r.getCompletion(id)
}

func (r *Repository) DeleteCompletion(completionID, habitID string) error {
	const q = `DELETE FROM habit_completions WHERE id = ? AND habit_id = ?`
	res, err := r.db.Exec(q, completionID, habitID)
	if err != nil {
		return fmt.Errorf("delete completion: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrCompletionNotFound
	}
	return nil
}

func (r *Repository) getCompletion(id string) (*Completion, error) {
	const q = `
		SELECT id, habit_id, completed_date, note, created_at
		FROM habit_completions WHERE id = ?`
	var c Completion
	var createdAt string
	err := r.db.QueryRow(q, id).Scan(
		&c.ID, &c.HabitID, &c.CompletedDate, &c.Note, &createdAt,
	)
	if err != nil {
		return nil, err
	}
	c.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	return &c, nil
}

// UpdateStreak writes the computed streak result back into the habits row.
// Called by the StreakJob after recalculating.
func (r *Repository) UpdateStreak(habitID string, result StreakResult) error {
	const q = `
		UPDATE habits
		SET current_streak      = ?,
		    longest_streak      = ?,
		    last_completed_date = ?
		WHERE id = ?`
	_, err := r.db.Exec(q,
		result.Current,
		result.Longest,
		result.LastCompletedDate,
		habitID,
	)
	return err
}

// ── Scan helpers ──────────────────────────────────────────────────────────────

type scanner interface {
	Scan(dest ...any) error
}

// scanHabitRow scans a single row (from QueryRow or Rows.Scan) into a *Habit.
// It handles both *sql.Row and *sql.Rows via the scanner interface.
func (r *Repository) scanHabitRow(row scanner) (*Habit, error) {
	var h Habit
	var freq string
	var scheduledDays sql.NullString
	var targetPerWeek sql.NullInt64
	var lastCompleted sql.NullString
	var createdAt, updatedAt string
	var isActive int

	err := row.Scan(
		&h.ID, &h.Title, &h.Description, &freq, &scheduledDays,
		&targetPerWeek, &h.Color, &h.Icon, &isActive,
		&h.CurrentStreak, &h.LongestStreak, &lastCompleted,
		&createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}

	h.Frequency = Frequency(freq)
	h.IsActive = isActive == 1
	h.ScheduledDays = DecodeDays(scheduledDays.String)

	if targetPerWeek.Valid {
		n := int(targetPerWeek.Int64)
		h.TargetPerWeek = &n
	}
	if lastCompleted.Valid {
		s := lastCompleted.String
		h.LastCompletedDate = &s
	}

	h.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	h.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

	return &h, nil
}
