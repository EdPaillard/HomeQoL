package habits

import (
	"time"
)

// This file contains the streak calculation logic.
//
// It is entirely pure — no DB access, no goroutines, no side effects.
// This makes it trivially testable: just call the functions with dates.
//
// The job in scheduler.go calls these functions and writes the results back
// to the database.

// ── Daily streak ──────────────────────────────────────────────────────────────

// CalcDailyStreak computes the current and longest streaks for a daily habit.
//
// Algorithm:
//  1. Sort completion dates descending (most recent first).
//  2. Starting from "yesterday" (today is still pending), walk backwards.
//     For each expected day, check if a completion exists.
//     Stop as soon as a day is missed.
//  3. The longest streak is the longest such run anywhere in history.
//
// The `today` parameter is injected so the function is deterministic in tests.
// In production, pass time.Now().In(loc) where loc is the user's timezone.
//
// `completedDates` is a slice of "YYYY-MM-DD" strings in any order.
func CalcDailyStreak(completedDates []string, today time.Time) StreakResult {
	if len(completedDates) == 0 {
		return StreakResult{}
	}

	// Build a set for O(1) lookup.
	dateSet := make(map[string]bool, len(completedDates))
	for _, d := range completedDates {
		dateSet[d] = true
	}

	todayStr := today.Format("2006-01-02")

	// ── Current streak ─────────────────────────────────────────────────────
	// Walk back from yesterday. If today is already completed, start from today.
	current := 0
	var lastCompletedDate *string

	startDay := today.AddDate(0, 0, -1) // yesterday
	if dateSet[todayStr] {
		startDay = today // today is done, count it
	}

	for d := startDay; ; d = d.AddDate(0, 0, -1) {
		key := d.Format("2006-01-02")
		if !dateSet[key] {
			break
		}
		current++
		if lastCompletedDate == nil {
			s := key
			lastCompletedDate = &s
		}
	}

	// ── Longest streak ─────────────────────────────────────────────────────
	// Walk all completion dates (not just recent ones) to find the longest run.
	// We do this by finding the earliest date and scanning forward.
	earliest, latest := dateRange(completedDates)
	longest := 0
	run := 0
	for d := earliest; !d.After(latest); d = d.AddDate(0, 0, 1) {
		if dateSet[d.Format("2006-01-02")] {
			run++
			if run > longest {
				longest = run
			}
		} else {
			run = 0
		}
	}
	// The current streak might be longer than anything in history
	// if all prior runs were shorter (e.g. first streak ever).
	if current > longest {
		longest = current
	}

	return StreakResult{
		Current:           current,
		Longest:           longest,
		LastCompletedDate: lastCompletedDate,
	}
}

// ── Weekly streak ─────────────────────────────────────────────────────────────

// CalcWeeklyStreak computes streaks for a habit that targets N completions
// per week (not necessarily every day).
//
// A "week" is an ISO week (Monday–Sunday).
// A week is "completed" if the number of completions in that week ≥ targetPerWeek.
//
// Algorithm:
//  1. Group all completions by ISO year+week.
//  2. Starting from last week (current week is still pending), walk backwards
//     through complete weeks. Count consecutive completed weeks.
//
// If targetPerWeek is 0 or negative, defaults to 1.
func CalcWeeklyStreak(completedDates []string, today time.Time, targetPerWeek int) StreakResult {
	if targetPerWeek <= 0 {
		targetPerWeek = 1
	}
	if len(completedDates) == 0 {
		return StreakResult{}
	}

	// Group completions by ISO year+week.
	type weekKey struct{ year, week int }
	weekCounts := make(map[weekKey]int, len(completedDates))
	for _, d := range completedDates {
		t, err := time.Parse("2006-01-02", d)
		if err != nil {
			continue
		}
		year, week := t.ISOWeek()
		weekCounts[weekKey{year, week}]++
	}

	weekCompleted := func(year, week int) bool {
		return weekCounts[weekKey{year, week}] >= targetPerWeek
	}

	// ── Current streak ─────────────────────────────────────────────────────
	// Start from last week; if this week is already completed, start from this week.
	thisYear, thisWeek := today.ISOWeek()
	lastWeek := today.AddDate(0, 0, -7)
	startYear, startWeek := lastWeek.ISOWeek()

	if weekCompleted(thisYear, thisWeek) {
		startYear, startWeek = thisYear, thisWeek
	}

	current := 0
	var lastCompletedDate *string

	// Walk back week by week.
	probe := isoWeekStart(startYear, startWeek)
	for {
		y, w := probe.ISOWeek()
		if !weekCompleted(y, w) {
			break
		}
		current++
		if lastCompletedDate == nil {
			// Use the last day of this week as the representative date.
			s := probe.AddDate(0, 0, 6).Format("2006-01-02")
			lastCompletedDate = &s
		}
		probe = probe.AddDate(0, 0, -7)
	}

	// ── Longest streak ─────────────────────────────────────────────────────
	// Collect all unique weeks that have completions, then scan for the
	// longest run of consecutive completed weeks.
	if len(weekCounts) == 0 {
		return StreakResult{Current: current, LastCompletedDate: lastCompletedDate}
	}

	// Find the earliest week.
	earliest, _ := dateRange(completedDates)
	earliestYear, earliestWeek := earliest.ISOWeek()

	longest := 0
	run := 0
	probe = isoWeekStart(earliestYear, earliestWeek)
	endProbe := isoWeekStart(thisYear, thisWeek)

	for !probe.After(endProbe) {
		y, w := probe.ISOWeek()
		if weekCompleted(y, w) {
			run++
			if run > longest {
				longest = run
			}
		} else {
			run = 0
		}
		probe = probe.AddDate(0, 0, 7)
	}
	if current > longest {
		longest = current
	}

	return StreakResult{
		Current:           current,
		Longest:           longest,
		LastCompletedDate: lastCompletedDate,
	}
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// dateRange returns the earliest and latest dates in the slice.
// The slice must be non-empty.
func dateRange(dates []string) (earliest, latest time.Time) {
	for i, d := range dates {
		t, err := time.Parse("2006-01-02", d)
		if err != nil {
			continue
		}
		if i == 0 || t.Before(earliest) {
			earliest = t
		}
		if i == 0 || t.After(latest) {
			latest = t
		}
	}
	return
}

// isoWeekStart returns the Monday (start of ISO week) for the given year+week.
func isoWeekStart(year, week int) time.Time {
	// Jan 4 is always in week 1.
	jan4 := time.Date(year, time.January, 4, 0, 0, 0, 0, time.UTC)
	_, jan4Week := jan4.ISOWeek()
	// Offset to Monday of week 1, then add (week-1) weeks.
	monday := jan4.AddDate(0, 0, -int(jan4.Weekday()-time.Monday))
	return monday.AddDate(0, 0, (week-jan4Week)*7)
}
