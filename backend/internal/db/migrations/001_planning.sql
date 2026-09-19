-- Migration 001: core planning tables
-- Tasks, goals, tags, and their relationships.
--
-- Design notes:
--   • All primary keys are UUIDs stored as TEXT — avoids auto-increment
--     collisions if you ever sync data across devices.
--   • Timestamps are stored as TEXT in ISO-8601 (RFC 3339) format.
--     SQLite has no native datetime type; TEXT is the most portable choice
--     and Go's time.Time marshals cleanly to/from it.
--   • Soft-delete via deleted_at: rows are never physically removed.
--     This preserves history and makes "undo delete" trivial.
--   • Foreign keys are enabled per-connection in Go (PRAGMA foreign_keys=ON).

-- ── Goals ────────────────────────────────────────────────────────────────────
-- A goal is a medium-to-long-term outcome (e.g. "Ship Planning Hub MVP").
-- It has milestones (child goals) and linked tasks.
CREATE TABLE IF NOT EXISTS goals (
    id          TEXT PRIMARY KEY,                        -- UUID v4
    parent_id   TEXT REFERENCES goals(id) ON DELETE SET NULL,  -- for milestones
    title       TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    status      TEXT NOT NULL DEFAULT 'active'           -- active | completed | abandoned
                CHECK (status IN ('active', 'completed', 'abandoned')),
    target_date TEXT,                                    -- ISO-8601 date, nullable
    completed_at TEXT,                                   -- set when status → completed
    created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    updated_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    deleted_at  TEXT                                     -- NULL = not deleted
);

CREATE INDEX IF NOT EXISTS idx_goals_status     ON goals(status) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_goals_parent_id  ON goals(parent_id);

-- ── Tasks ────────────────────────────────────────────────────────────────────
-- A task is a single unit of work with an optional due date, priority,
-- and optional linkage to a goal.
CREATE TABLE IF NOT EXISTS tasks (
    id          TEXT PRIMARY KEY,
    goal_id     TEXT REFERENCES goals(id) ON DELETE SET NULL,
    title       TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    status      TEXT NOT NULL DEFAULT 'todo'
                CHECK (status IN ('todo', 'in_progress', 'done', 'cancelled')),
    priority    INTEGER NOT NULL DEFAULT 2              -- 1=high 2=medium 3=low
                CHECK (priority IN (1, 2, 3)),
    due_date    TEXT,                                   -- ISO-8601 date, nullable
    completed_at TEXT,
    created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    updated_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    deleted_at  TEXT
);

CREATE INDEX IF NOT EXISTS idx_tasks_status     ON tasks(status)   WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_tasks_due_date   ON tasks(due_date) WHERE deleted_at IS NULL AND due_date IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_tasks_goal_id    ON tasks(goal_id)  WHERE goal_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_tasks_priority   ON tasks(priority) WHERE deleted_at IS NULL;

-- ── Tags ─────────────────────────────────────────────────────────────────────
-- Free-form labels shared across tasks (and later habits, goals).
CREATE TABLE IF NOT EXISTS tags (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL UNIQUE,                    -- e.g. "devops", "health"
    color      TEXT NOT NULL DEFAULT '#888888',         -- hex color for UI
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

-- Many-to-many: tasks ↔ tags
CREATE TABLE IF NOT EXISTS task_tags (
    task_id TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    tag_id  TEXT NOT NULL REFERENCES tags(id)  ON DELETE CASCADE,
    PRIMARY KEY (task_id, tag_id)
);

CREATE INDEX IF NOT EXISTS idx_task_tags_tag_id ON task_tags(tag_id);

-- ── updated_at triggers ──────────────────────────────────────────────────────
-- SQLite doesn't have ON UPDATE hooks on columns, so we use triggers
-- to keep updated_at accurate without burdening the application layer.
CREATE TRIGGER IF NOT EXISTS goals_updated_at
    AFTER UPDATE ON goals
    FOR EACH ROW
    WHEN OLD.updated_at = NEW.updated_at   -- only fire if app didn't set it
    BEGIN
        UPDATE goals SET updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
        WHERE id = NEW.id;
    END;

CREATE TRIGGER IF NOT EXISTS tasks_updated_at
    AFTER UPDATE ON tasks
    FOR EACH ROW
    WHEN OLD.updated_at = NEW.updated_at
    BEGIN
        UPDATE tasks SET updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
        WHERE id = NEW.id;
    END;
