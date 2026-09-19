-- Migration 005: migration tracking + seed data
--
-- This migration is special: it creates the table that the Go migration runner
-- uses to know which migrations have already been applied.
-- It is always run first (version 0), before any numbered migration.
--
-- Seed data inserts sensible defaults so the app is usable on first launch
-- without manual setup.

-- ── Migration tracking ───────────────────────────────────────────────────────
-- The Go runner checks this table before applying each migration file.
-- If a filename is already present, the migration is skipped.
CREATE TABLE IF NOT EXISTS schema_migrations (
    filename    TEXT PRIMARY KEY,                       -- e.g. "001_planning.sql"
    applied_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

-- ── Seed: default envelope templates ────────────────────────────────────────
-- These cover a typical French household. Adjust amounts and names to taste.
-- All amounts are in euro cents.
INSERT OR IGNORE INTO envelope_templates (id, name, budgeted_cents, color, icon, sort_order) VALUES
    ('env-tpl-001', 'Loyer / Crédit',        120000, '#7A9E87', '🏠', 1),
    ('env-tpl-002', 'Courses',                40000, '#C97B6E', '🛒', 2),
    ('env-tpl-003', 'Transports',             15000, '#7E9FC2', '🚗', 3),
    ('env-tpl-004', 'Restaurants / Sorties',  15000, '#C9A256', '🍽', 4),
    ('env-tpl-005', 'Santé',                  10000, '#9E7ABF', '💊', 5),
    ('env-tpl-006', 'Électricité',             8000, '#F0A500', '⚡', 6),
    ('env-tpl-007', 'Abonnements',             5000, '#5AABB8', '📱', 7),
    ('env-tpl-008', 'Épargne',                30000, '#1D9E75', '🏦', 8),
    ('env-tpl-009', 'Loisirs',                10000, '#D85A30', '🎮', 9),
    ('env-tpl-010', 'Divers',                  5000, '#888780', '📦', 10);

-- ── Seed: categorisation rules ───────────────────────────────────────────────
-- Patterns matched (case-insensitively in Go) against the transaction label.
-- sort_order: lower = higher priority. Add your own as you discover patterns.
INSERT OR IGNORE INTO categorisation_rules (id, template_id, pattern, match_type, sort_order) VALUES
    -- Electricity (Linky auto-inject also uses this template_id)
    ('rule-001', 'env-tpl-006', 'EDF',             'contains',    10),
    ('rule-002', 'env-tpl-006', 'ENEDIS',          'contains',    11),
    ('rule-003', 'env-tpl-006', 'DIRECT ENERGIE',  'contains',    12),
    -- Groceries
    ('rule-010', 'env-tpl-002', 'LECLERC',         'contains',    20),
    ('rule-011', 'env-tpl-002', 'CARREFOUR',       'contains',    21),
    ('rule-012', 'env-tpl-002', 'INTERMARCHE',     'contains',    22),
    ('rule-013', 'env-tpl-002', 'LIDL',            'contains',    23),
    ('rule-014', 'env-tpl-002', 'ALDI',            'contains',    24),
    ('rule-015', 'env-tpl-002', 'MONOPRIX',        'contains',    25),
    -- Transport
    ('rule-020', 'env-tpl-003', 'SNCF',            'contains',    30),
    ('rule-021', 'env-tpl-003', 'RATP',            'contains',    31),
    ('rule-022', 'env-tpl-003', 'TOTAL',           'contains',    32),
    ('rule-023', 'env-tpl-003', 'BP ',             'starts_with', 33),
    ('rule-024', 'env-tpl-003', 'AUTOROUTE',       'contains',    34),
    -- Subscriptions
    ('rule-030', 'env-tpl-007', 'NETFLIX',         'contains',    40),
    ('rule-031', 'env-tpl-007', 'SPOTIFY',         'contains',    41),
    ('rule-032', 'env-tpl-007', 'AMAZON PRIME',    'contains',    42),
    ('rule-033', 'env-tpl-007', 'FREE',            'contains',    43),
    ('rule-034', 'env-tpl-007', 'ORANGE',          'contains',    44),
    ('rule-035', 'env-tpl-007', 'SFR',             'contains',    45);

-- ── Seed: default tags ───────────────────────────────────────────────────────
INSERT OR IGNORE INTO tags (id, name, color) VALUES
    ('tag-001', 'personnel-os',  '#7A9E87'),
    ('tag-002', 'devops',        '#7E9FC2'),
    ('tag-003', 'go',            '#00ACD7'),
    ('tag-004', 'react',         '#61DAFB'),
    ('tag-005', 'flutter',       '#54C5F8'),
    ('tag-006', 'santé',         '#C97B6E'),
    ('tag-007', 'maison',        '#C9A256'),
    ('tag-008', 'urgent',        '#D85A30');
