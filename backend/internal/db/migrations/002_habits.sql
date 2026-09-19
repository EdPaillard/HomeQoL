-- Migration 002: habits
--
-- Design notes:
--   • habits defines WHAT to do and HOW OFTEN (frequency + schedule).
--   • habit_completions is an append-only log — one row per day per habit.
--     Never update completions; if a user "un-checks" a habit, delete the row.
--   • Streaks are stored denormalised in habits for fast reads.
--     They are recomputed by a nightly Go goroutine (not by triggers) because
--     streak logic depends on "today's date", which triggers can't know.
--   • scheduled_days is a comma-separated list of ISO weekday numbers
--     (1=Monday … 7=Sunday) for weekly habits, e.g. "1,3,5" = Mon/Wed/Fri.
--     NULL means "every day" for daily habits.

-- ── Habits ───────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS habits (
    id              TEXT PRIMARY KEY,
    title           TEXT NOT NULL,
    description     TEXT NOT NULL DEFAULT '',
    frequency       TEXT NOT NULL DEFAULT 'daily'
                    CHECK (frequency IN ('daily', 'weekly')),
    scheduled_days  TEXT,                               -- NULL=daily, "1,3,5"=Mon/Wed/Fri
    target_per_week INTEGER,                            -- e.g. 3 = "at least 3×/week"
    color           TEXT NOT NULL DEFAULT '#1D9E75',    -- UI accent color
    icon            TEXT NOT NULL DEFAULT '✓',          -- emoji or icon name
    is_active       INTEGER NOT NULL DEFAULT 1          -- 1=active, 0=archived
                    CHECK (is_active IN (0, 1)),
    -- Denormalised streak data (recomputed nightly)
    current_streak  INTEGER NOT NULL DEFAULT 0,
    longest_streak  INTEGER NOT NULL DEFAULT 0,
    last_completed_date TEXT,                           -- ISO-8601 date
    created_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    updated_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    deleted_at      TEXT
);

CREATE INDEX IF NOT EXISTS idx_habits_active ON habits(is_active) WHERE deleted_at IS NULL;

-- ── Habit completions ────────────────────────────────────────────────────────
-- Append-only log. One row = "this habit was completed on this date".
-- The unique constraint prevents double-logging the same day.
CREATE TABLE IF NOT EXISTS habit_completions (
    id          TEXT PRIMARY KEY,
    habit_id    TEXT NOT NULL REFERENCES habits(id) ON DELETE CASCADE,
    completed_date TEXT NOT NULL,                       -- ISO-8601 date "YYYY-MM-DD"
    note        TEXT NOT NULL DEFAULT '',               -- optional journal note
    created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),

    UNIQUE (habit_id, completed_date)                   -- one entry per habit per day
);

CREATE INDEX IF NOT EXISTS idx_habit_completions_habit_id ON habit_completions(habit_id);
CREATE INDEX IF NOT EXISTS idx_habit_completions_date     ON habit_completions(completed_date);

-- Composite index used by the streak-recompute query:
-- SELECT * FROM habit_completions WHERE habit_id = ? ORDER BY completed_date DESC
CREATE INDEX IF NOT EXISTS idx_habit_completions_habit_date
    ON habit_completions(habit_id, completed_date DESC);

-- ── updated_at trigger ───────────────────────────────────────────────────────
CREATE TRIGGER IF NOT EXISTS habits_updated_at
    AFTER UPDATE ON habits
    FOR EACH ROW
    WHEN OLD.updated_at = NEW.updated_at
    BEGIN
        UPDATE habits SET updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
        WHERE id = NEW.id;
    END;
