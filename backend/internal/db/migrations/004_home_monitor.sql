-- Migration 004: home monitor — Tydom + Linky time-series data
--
-- Design notes:
--   • All readings tables are append-only. Rows are NEVER updated.
--     This is the correct model for sensor time-series data.
--   • recorded_at is the real-world time the reading occurred, not created_at.
--     They differ when data arrives delayed (Linky sends yesterday's data at 8h).
--   • SQLite is not a time-series database, but it handles this workload well
--     up to ~millions of rows. If you ever need more, swap to TimescaleDB or
--     InfluxDB behind the same Go service interface.
--   • Linky data arrives once per day; Tydom data arrives in real-time via MQTT.
--     Separate tables reflect their different cadences and schemas.

-- ── Tydom rooms ──────────────────────────────────────────────────────────────
-- A room (or zone) as configured in your Tydom app.
-- Populated on first sync from the tydom2mqtt MQTT topics.
CREATE TABLE IF NOT EXISTS tydom_rooms (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL UNIQUE,                   -- "Salon", "Chambre 1"…
    external_id TEXT,                                   -- Tydom zone ID if available
    created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

-- ── Tydom temperature readings ───────────────────────────────────────────────
-- One row per temperature measurement per room.
-- INSERT only — never UPDATE or DELETE.
CREATE TABLE IF NOT EXISTS tydom_temperature_readings (
    id          TEXT PRIMARY KEY,
    room_id     TEXT NOT NULL REFERENCES tydom_rooms(id) ON DELETE CASCADE,
    celsius     REAL NOT NULL,
    source      TEXT NOT NULL DEFAULT 'interior'
                CHECK (source IN ('interior', 'exterior')),
    recorded_at TEXT NOT NULL,                          -- ISO-8601 datetime
    created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

-- Primary query: temperature history for a room over a time window
CREATE INDEX IF NOT EXISTS idx_temp_room_time
    ON tydom_temperature_readings(room_id, recorded_at DESC);

-- Query: latest exterior temperature (for correlation with energy data)
CREATE INDEX IF NOT EXISTS idx_temp_exterior_time
    ON tydom_temperature_readings(recorded_at DESC)
    WHERE source = 'exterior';

-- ── Tydom shutter readings ───────────────────────────────────────────────────
-- Records shutter position changes. position: 0=fully closed, 100=fully open.
CREATE TABLE IF NOT EXISTS tydom_shutter_readings (
    id          TEXT PRIMARY KEY,
    room_id     TEXT NOT NULL REFERENCES tydom_rooms(id) ON DELETE CASCADE,
    position    INTEGER NOT NULL CHECK (position BETWEEN 0 AND 100),
    recorded_at TEXT NOT NULL,
    created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

CREATE INDEX IF NOT EXISTS idx_shutter_room_time
    ON tydom_shutter_readings(room_id, recorded_at DESC);

-- ── Tydom sunlight readings ──────────────────────────────────────────────────
-- Sunlight level from the Tydom exterior sensor, used for shutter automation
-- and energy consumption correlation.
CREATE TABLE IF NOT EXISTS tydom_sunlight_readings (
    id          TEXT PRIMARY KEY,
    lux         REAL NOT NULL,
    recorded_at TEXT NOT NULL,
    created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

CREATE INDEX IF NOT EXISTS idx_sunlight_time
    ON tydom_sunlight_readings(recorded_at DESC);

-- ── Linky energy readings ────────────────────────────────────────────────────
-- Daily energy consumption fetched from Enedis via Conso API.
-- granularity: 'daily' for day totals, 'hourly' for half-hour slots.
-- kwh is stored as REAL (not cents) because energy has meaningful decimal values.
CREATE TABLE IF NOT EXISTS linky_energy_readings (
    id              TEXT PRIMARY KEY,
    reading_date    TEXT NOT NULL,                      -- ISO-8601 date "YYYY-MM-DD"
    reading_hour    INTEGER,                            -- 0-23, NULL for daily totals
    kwh             REAL NOT NULL,
    granularity     TEXT NOT NULL DEFAULT 'daily'
                    CHECK (granularity IN ('daily', 'hourly')),
    created_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),

    UNIQUE (reading_date, reading_hour, granularity)    -- prevent duplicate imports
);

CREATE INDEX IF NOT EXISTS idx_linky_date
    ON linky_energy_readings(reading_date DESC, granularity);

-- ── Alerts ───────────────────────────────────────────────────────────────────
-- Fired when a threshold is crossed (temperature too low, energy spike, etc.).
-- acknowledged_at: NULL = unread, set when the user dismisses it.
CREATE TABLE IF NOT EXISTS alerts (
    id              TEXT PRIMARY KEY,
    type            TEXT NOT NULL
                    CHECK (type IN ('temperature_low', 'temperature_high',
                                    'energy_spike', 'shutter_open_rain',
                                    'custom')),
    severity        TEXT NOT NULL DEFAULT 'info'
                    CHECK (severity IN ('info', 'warning', 'critical')),
    title           TEXT NOT NULL,
    body            TEXT NOT NULL DEFAULT '',
    source_table    TEXT,                               -- e.g. "tydom_temperature_readings"
    source_id       TEXT,                               -- FK to the row that triggered it
    acknowledged_at TEXT,
    created_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

CREATE INDEX IF NOT EXISTS idx_alerts_unread
    ON alerts(created_at DESC)
    WHERE acknowledged_at IS NULL;
