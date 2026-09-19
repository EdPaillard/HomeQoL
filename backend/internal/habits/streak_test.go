package habits_test

import (
	"testing"
	"time"

	"homeqol/internal/habits"
)

// reference date: a Wednesday
var today = time.Date(2025, 5, 7, 23, 59, 0, 0, time.UTC) // Wednesday 7 May 2025

func date(s string) string { return s } // identity — just for readability

// ── CalcDailyStreak ───────────────────────────────────────────────────────────

func TestDailyStreak_noCompletions(t *testing.T) {
	r := habits.CalcDailyStreak(nil, today)
	if r.Current != 0 || r.Longest != 0 {
		t.Errorf("expected 0/0, got %d/%d", r.Current, r.Longest)
	}
}

func TestDailyStreak_completedTodayOnly(t *testing.T) {
	dates := []string{date("2025-05-07")} // today
	r := habits.CalcDailyStreak(dates, today)
	if r.Current != 1 {
		t.Errorf("expected current=1, got %d", r.Current)
	}
	if r.Longest != 1 {
		t.Errorf("expected longest=1, got %d", r.Longest)
	}
}

func TestDailyStreak_completedYesterdayOnly(t *testing.T) {
	dates := []string{date("2025-05-06")} // yesterday
	r := habits.CalcDailyStreak(dates, today)
	if r.Current != 1 {
		t.Errorf("expected current=1, got %d", r.Current)
	}
}

func TestDailyStreak_missedYesterdayBreaksStreak(t *testing.T) {
	// Today done, yesterday missed, day before done.
	dates := []string{"2025-05-07", "2025-05-05"}
	r := habits.CalcDailyStreak(dates, today)
	// Current streak is 1 (only today), because yesterday was missed.
	if r.Current != 1 {
		t.Errorf("expected current=1 (gap on yesterday), got %d", r.Current)
	}
	// Longest run is also 1 (no consecutive days).
	if r.Longest != 1 {
		t.Errorf("expected longest=1, got %d", r.Longest)
	}
}

func TestDailyStreak_sevenDayStreak(t *testing.T) {
	dates := []string{
		"2025-05-07", // today (Wed)
		"2025-05-06", // Tue
		"2025-05-05", // Mon
		"2025-05-04", // Sun
		"2025-05-03", // Sat
		"2025-05-02", // Fri
		"2025-05-01", // Thu
	}
	r := habits.CalcDailyStreak(dates, today)
	if r.Current != 7 {
		t.Errorf("expected current=7, got %d", r.Current)
	}
	if r.Longest != 7 {
		t.Errorf("expected longest=7, got %d", r.Longest)
	}
}

func TestDailyStreak_longestPreservesHistoricalRecord(t *testing.T) {
	// 10-day streak two months ago, only 3-day streak now.
	var dates []string
	// Historical 10-day run: 2025-01-01 to 2025-01-10
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 10; i++ {
		dates = append(dates, base.AddDate(0, 0, i).Format("2006-01-02"))
	}
	// Current 3-day run: today and 2 days before
	dates = append(dates, "2025-05-07", "2025-05-06", "2025-05-05")

	r := habits.CalcDailyStreak(dates, today)
	if r.Current != 3 {
		t.Errorf("expected current=3, got %d", r.Current)
	}
	if r.Longest != 10 {
		t.Errorf("expected longest=10 (historical), got %d", r.Longest)
	}
}

func TestDailyStreak_notCompletedTodayStreakStillLive(t *testing.T) {
	// Yesterday completed, today not yet — streak should still be 1 (not broken).
	dates := []string{"2025-05-06"} // only yesterday
	r := habits.CalcDailyStreak(dates, today)
	if r.Current != 1 {
		t.Errorf("expected streak=1 (today still pending), got %d", r.Current)
	}
}

func TestDailyStreak_lastCompletedDateIsSet(t *testing.T) {
	dates := []string{"2025-05-07", "2025-05-06"}
	r := habits.CalcDailyStreak(dates, today)
	if r.LastCompletedDate == nil {
		t.Fatal("expected LastCompletedDate to be set")
	}
	if *r.LastCompletedDate != "2025-05-07" {
		t.Errorf("expected 2025-05-07, got %s", *r.LastCompletedDate)
	}
}

func TestDailyStreak_unsortedInputProducesCorrectResult(t *testing.T) {
	// Input order should not affect the result.
	dates := []string{"2025-05-05", "2025-05-07", "2025-05-06"}
	r := habits.CalcDailyStreak(dates, today)
	if r.Current != 3 {
		t.Errorf("expected current=3, got %d", r.Current)
	}
}

// ── CalcWeeklyStreak ──────────────────────────────────────────────────────────

// ISO week: Mon 5 May 2025 = week 19, 2025.
// Last week = week 18 (Mon 28 Apr – Sun 4 May).

func TestWeeklyStreak_noCompletions(t *testing.T) {
	r := habits.CalcWeeklyStreak(nil, today, 3)
	if r.Current != 0 || r.Longest != 0 {
		t.Errorf("expected 0/0, got %d/%d", r.Current, r.Longest)
	}
}

func TestWeeklyStreak_targetMetThisWeekOnly(t *testing.T) {
	// 3 completions this week (target=3) — current streak should be 1.
	dates := []string{"2025-05-05", "2025-05-06", "2025-05-07"}
	r := habits.CalcWeeklyStreak(dates, today, 3)
	if r.Current != 1 {
		t.Errorf("expected current=1, got %d", r.Current)
	}
}

func TestWeeklyStreak_targetNotMetThisWeek(t *testing.T) {
	// Only 2 completions this week, target is 3. Last week had 3.
	dates := []string{
		"2025-04-28", "2025-04-29", "2025-04-30", // last week: 3 ✓
		"2025-05-05", "2025-05-06", // this week: 2 (pending)
	}
	r := habits.CalcWeeklyStreak(dates, today, 3)
	// This week is still in progress (it's Wed); last week was complete → streak=1.
	if r.Current != 1 {
		t.Errorf("expected current=1 (last week done, this week pending), got %d", r.Current)
	}
}

func TestWeeklyStreak_threeConsecutiveWeeks(t *testing.T) {
	dates := []string{
		// Week 17 (21–27 Apr): 3 completions
		"2025-04-21", "2025-04-22", "2025-04-23",
		// Week 18 (28 Apr – 4 May): 3 completions
		"2025-04-28", "2025-04-29", "2025-04-30",
		// Week 19 (5–11 May, this week): 3 completions
		"2025-05-05", "2025-05-06", "2025-05-07",
	}
	r := habits.CalcWeeklyStreak(dates, today, 3)
	if r.Current != 3 {
		t.Errorf("expected current=3, got %d", r.Current)
	}
	if r.Longest != 3 {
		t.Errorf("expected longest=3, got %d", r.Longest)
	}
}

func TestWeeklyStreak_gapWeekResetsStreak(t *testing.T) {
	dates := []string{
		// Week 17: 3 ✓
		"2025-04-21", "2025-04-22", "2025-04-23",
		// Week 18: 0 ✗ (gap!)
		// Week 19: 3 ✓
		"2025-05-05", "2025-05-06", "2025-05-07",
	}
	r := habits.CalcWeeklyStreak(dates, today, 3)
	// Gap on week 18 resets streak. Current = 1 (only this week).
	if r.Current != 1 {
		t.Errorf("expected current=1 after gap, got %d", r.Current)
	}
	// Longest was 1 (week 17 was a single-week run before the gap).
	if r.Longest != 1 {
		t.Errorf("expected longest=1, got %d", r.Longest)
	}
}

func TestWeeklyStreak_targetOneDefaultsCorrectly(t *testing.T) {
	dates := []string{"2025-05-07"}
	// target=0 should be treated as 1
	r := habits.CalcWeeklyStreak(dates, today, 0)
	if r.Current != 1 {
		t.Errorf("expected current=1 with default target, got %d", r.Current)
	}
}
