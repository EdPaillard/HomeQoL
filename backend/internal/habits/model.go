// Package habits implements the habit tracking domain.
//
// Key concepts:
//   - A Habit is a recurring behaviour with a frequency (daily or weekly).
//   - A Completion is an immutable log entry: "this habit was done on this date".
//   - A Streak is the count of consecutive periods (days or weeks) the habit
//     was completed without a miss. It is computed from the completion log and
//     stored denormalised on the Habit row for fast reads.
//   - The StreakJob recalculates all active habits nightly.
package habits

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ── Frequency ─────────────────────────────────────────────────────────────────

type Frequency string

const (
	FrequencyDaily  Frequency = "daily"
	FrequencyWeekly Frequency = "weekly"
)

func (f Frequency) Valid() bool {
	return f == FrequencyDaily || f == FrequencyWeekly
}

// ── Habit ─────────────────────────────────────────────────────────────────────

type Habit struct {
	ID            string    `json:"id"`
	Title         string    `json:"title"`
	Description   string    `json:"description"`
	Frequency     Frequency `json:"frequency"`
	ScheduledDays []int     `json:"scheduled_days"` // 1=Mon…7=Sun; empty = every day
	TargetPerWeek *int      `json:"target_per_week,omitempty"`
	Color         string    `json:"color"`
	Icon          string    `json:"icon"`
	IsActive      bool      `json:"is_active"`

	// Denormalised — written by StreakJob, read by the API.
	CurrentStreak     int     `json:"current_streak"`
	LongestStreak     int     `json:"longest_streak"`
	LastCompletedDate *string `json:"last_completed_date,omitempty"`

	// Computed on read — not stored.
	CompletedToday bool `json:"completed_today"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// IsScheduledOn returns true if the habit should be done on the given weekday.
// For daily habits (no scheduled days), always returns true.
func (h *Habit) IsScheduledOn(weekday time.Weekday) bool {
	if len(h.ScheduledDays) == 0 {
		return true // daily
	}
	// time.Weekday: Sunday=0, Monday=1 … Saturday=6
	// Our schema: Monday=1 … Sunday=7
	isoDay := int(weekday)
	if isoDay == 0 {
		isoDay = 7 // convert Sunday
	}
	for _, d := range h.ScheduledDays {
		if d == isoDay {
			return true
		}
	}
	return false
}

// ── Completion ────────────────────────────────────────────────────────────────

type Completion struct {
	ID            string    `json:"id"`
	HabitID       string    `json:"habit_id"`
	CompletedDate string    `json:"completed_date"` // "YYYY-MM-DD"
	Note          string    `json:"note"`
	CreatedAt     time.Time `json:"created_at"`
}

// ── Streak calculation types ──────────────────────────────────────────────────

// StreakResult is produced by the pure streak calculation functions.
// Storing it separately from Habit makes the calculation easy to test.
type StreakResult struct {
	Current           int
	Longest           int
	LastCompletedDate *string
}

// ── Request / response DTOs ───────────────────────────────────────────────────

type CreateRequest struct {
	Title         string    `json:"title"`
	Description   string    `json:"description"`
	Frequency     Frequency `json:"frequency"`
	ScheduledDays []int     `json:"scheduled_days"` // empty = every day
	TargetPerWeek *int      `json:"target_per_week"`
	Color         string    `json:"color"`
	Icon          string    `json:"icon"`
}

func (r *CreateRequest) Validate() error {
	r.Title = strings.TrimSpace(r.Title)
	if r.Title == "" {
		return errors.New("title is required")
	}
	if r.Frequency == "" {
		r.Frequency = FrequencyDaily
	}
	if !r.Frequency.Valid() {
		return errors.New("frequency must be 'daily' or 'weekly'")
	}
	if r.Color == "" {
		r.Color = "#1D9E75"
	}
	if r.Icon == "" {
		r.Icon = "✓"
	}
	for _, d := range r.ScheduledDays {
		if d < 1 || d > 7 {
			return fmt.Errorf("scheduled_days must be 1–7, got %d", d)
		}
	}
	if r.Frequency == FrequencyWeekly && r.TargetPerWeek != nil {
		if *r.TargetPerWeek < 1 || *r.TargetPerWeek > 7 {
			return errors.New("target_per_week must be between 1 and 7")
		}
	}
	return nil
}

type UpdateRequest struct {
	Title         *string    `json:"title"`
	Description   *string    `json:"description"`
	Frequency     *Frequency `json:"frequency"`
	ScheduledDays []int      `json:"scheduled_days"`
	TargetPerWeek *int       `json:"target_per_week"`
	Color         *string    `json:"color"`
	Icon          *string    `json:"icon"`
	IsActive      *bool      `json:"is_active"`
}

func (r *UpdateRequest) Validate() error {
	if r.Title != nil {
		*r.Title = strings.TrimSpace(*r.Title)
		if *r.Title == "" {
			return errors.New("title cannot be empty")
		}
	}
	if r.Frequency != nil && !r.Frequency.Valid() {
		return errors.New("frequency must be 'daily' or 'weekly'")
	}
	return nil
}

type CompleteRequest struct {
	Date string `json:"date"` // "YYYY-MM-DD"; defaults to today if empty
	Note string `json:"note"`
}

func (r *CompleteRequest) Validate(now time.Time) error {
	if r.Date == "" {
		r.Date = now.Format("2006-01-02")
		return nil
	}
	if _, err := time.Parse("2006-01-02", r.Date); err != nil {
		return errors.New("date must be in YYYY-MM-DD format")
	}
	// Don't allow backdating more than 7 days (prevents data manipulation).
	parsed, _ := time.Parse("2006-01-02", r.Date)
	if parsed.Before(now.AddDate(0, 0, -7)) {
		return errors.New("cannot log a completion more than 7 days in the past")
	}
	return nil
}

// ── scheduledDays encoding ────────────────────────────────────────────────────
// The DB stores scheduled_days as a comma-separated string: "1,3,5"
// These helpers convert between []int and string.

func EncodeDays(days []int) string {
	if len(days) == 0 {
		return ""
	}
	parts := make([]string, len(days))
	for i, d := range days {
		parts[i] = strconv.Itoa(d)
	}
	return strings.Join(parts, ",")
}

func DecodeDays(s string) []int {
	if s == "" {
		return []int{}
	}
	parts := strings.Split(s, ",")
	days := make([]int, 0, len(parts))
	for _, p := range parts {
		if n, err := strconv.Atoi(strings.TrimSpace(p)); err == nil {
			days = append(days, n)
		}
	}
	return days
}

// ── Sentinel errors ───────────────────────────────────────────────────────────

var (
	ErrNotFound           = errors.New("habit not found")
	ErrAlreadyDone        = errors.New("habit already completed for this date")
	ErrCompletionNotFound = errors.New("completion not found")
)
