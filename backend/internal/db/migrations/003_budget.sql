-- Migration 003: budget
--
-- Design notes:
--   • budget_periods are monthly containers (Jan 2025, Feb 2025…).
--     Envelopes are created fresh each period by copying from templates.
--   • envelope_templates defines the recurring envelopes (Groceries, Rent…)
--     so creating a new period auto-populates its envelopes.
--   • bank_accounts mirrors account data synced from Powens/Caisse d'Épargne.
--     This is a local cache — the source of truth is the bank.
--   • transactions are imported from the bank + any manual entries.
--     amount is stored in EURO CENTS (INTEGER) to avoid floating-point rounding.
--     A purchase of 42,50 € is stored as 4250.
--   • category_rule stores the pattern that auto-categorised the transaction
--     so you can audit and improve the rules over time.

-- ── Bank accounts (synced from Powens) ──────────────────────────────────────
CREATE TABLE IF NOT EXISTS bank_accounts (
    id              TEXT PRIMARY KEY,
    external_id     TEXT NOT NULL UNIQUE,               -- Powens account ID
    label           TEXT NOT NULL,                      -- "Compte Courant", "Livret A"…
    type            TEXT NOT NULL DEFAULT 'checking'
                    CHECK (type IN ('checking', 'savings', 'credit', 'other')),
    balance_cents   INTEGER NOT NULL DEFAULT 0,         -- in euro cents, updated on sync
    currency        TEXT NOT NULL DEFAULT 'EUR',
    iban            TEXT,
    last_synced_at  TEXT,
    created_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    updated_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

-- ── Envelope templates ───────────────────────────────────────────────────────
-- The standing list of envelopes you use every month.
-- When a new budget_period is created, these are copied into budget_envelopes.
CREATE TABLE IF NOT EXISTS envelope_templates (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL UNIQUE,               -- "Groceries", "Rent", "Electricity"
    budgeted_cents  INTEGER NOT NULL DEFAULT 0,         -- default monthly budget
    color           TEXT NOT NULL DEFAULT '#7A9E87',
    icon            TEXT NOT NULL DEFAULT '💶',
    sort_order      INTEGER NOT NULL DEFAULT 0,
    is_active       INTEGER NOT NULL DEFAULT 1
                    CHECK (is_active IN (0, 1)),
    created_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

-- ── Budget periods ───────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS budget_periods (
    id          TEXT PRIMARY KEY,
    year        INTEGER NOT NULL,
    month       INTEGER NOT NULL CHECK (month BETWEEN 1 AND 12),
    is_closed   INTEGER NOT NULL DEFAULT 0,             -- 1 once the month is finalised
    created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),

    UNIQUE (year, month)
);

-- ── Budget envelopes ─────────────────────────────────────────────────────────
-- One row per envelope per period. Copied from envelope_templates each month.
-- spent_cents is a denormalised sum recomputed whenever a transaction changes.
CREATE TABLE IF NOT EXISTS budget_envelopes (
    id              TEXT PRIMARY KEY,
    period_id       TEXT NOT NULL REFERENCES budget_periods(id) ON DELETE CASCADE,
    template_id     TEXT REFERENCES envelope_templates(id) ON DELETE SET NULL,
    name            TEXT NOT NULL,                      -- copied from template at creation
    budgeted_cents  INTEGER NOT NULL DEFAULT 0,
    spent_cents     INTEGER NOT NULL DEFAULT 0,         -- recomputed, never set manually
    color           TEXT NOT NULL DEFAULT '#7A9E87',
    icon            TEXT NOT NULL DEFAULT '💶',
    sort_order      INTEGER NOT NULL DEFAULT 0,
    created_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    updated_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),

    UNIQUE (period_id, name)
);

CREATE INDEX IF NOT EXISTS idx_envelopes_period ON budget_envelopes(period_id);

-- ── Transactions ─────────────────────────────────────────────────────────────
-- Imported from Powens or entered manually.
-- amount_cents: negative = expense (money out), positive = income (money in).
CREATE TABLE IF NOT EXISTS transactions (
    id              TEXT PRIMARY KEY,
    account_id      TEXT REFERENCES bank_accounts(id) ON DELETE SET NULL,
    envelope_id     TEXT REFERENCES budget_envelopes(id) ON DELETE SET NULL,
    external_id     TEXT UNIQUE,                        -- Powens transaction ID, NULL if manual
    label           TEXT NOT NULL,                      -- original bank label
    clean_label     TEXT NOT NULL DEFAULT '',           -- normalised label for display
    amount_cents    INTEGER NOT NULL,                   -- negative=expense, positive=income
    currency        TEXT NOT NULL DEFAULT 'EUR',
    transaction_date TEXT NOT NULL,                     -- ISO-8601 date "YYYY-MM-DD"
    category_rule   TEXT,                               -- which rule matched, for audit
    source          TEXT NOT NULL DEFAULT 'bank'
                    CHECK (source IN ('bank', 'manual')),
    note            TEXT NOT NULL DEFAULT '',           -- user annotation
    created_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    updated_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

CREATE INDEX IF NOT EXISTS idx_transactions_date        ON transactions(transaction_date DESC);
CREATE INDEX IF NOT EXISTS idx_transactions_envelope    ON transactions(envelope_id) WHERE envelope_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_transactions_account     ON transactions(account_id)  WHERE account_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_transactions_external_id ON transactions(external_id) WHERE external_id IS NOT NULL;

-- ── Categorisation rules ─────────────────────────────────────────────────────
-- Ordered list of label-matching rules that auto-assign transactions to envelopes.
-- The Go service evaluates rules in sort_order ASC; first match wins.
CREATE TABLE IF NOT EXISTS categorisation_rules (
    id              TEXT PRIMARY KEY,
    template_id     TEXT NOT NULL REFERENCES envelope_templates(id) ON DELETE CASCADE,
    pattern         TEXT NOT NULL,                      -- substring or regex to match against label
    match_type      TEXT NOT NULL DEFAULT 'contains'
                    CHECK (match_type IN ('contains', 'starts_with', 'regex')),
    sort_order      INTEGER NOT NULL DEFAULT 0,
    is_active       INTEGER NOT NULL DEFAULT 1
                    CHECK (is_active IN (0, 1)),
    created_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

CREATE INDEX IF NOT EXISTS idx_rules_active_order ON categorisation_rules(sort_order) WHERE is_active = 1;

-- ── Trigger: recompute spent_cents on envelope after transaction change ───────
-- Keeps spent_cents accurate without a round-trip query from Go.
CREATE TRIGGER IF NOT EXISTS trg_transactions_update_envelope_insert
    AFTER INSERT ON transactions
    WHEN NEW.envelope_id IS NOT NULL AND NEW.amount_cents < 0
    BEGIN
        UPDATE budget_envelopes
        SET spent_cents = spent_cents + ABS(NEW.amount_cents),
            updated_at  = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
        WHERE id = NEW.envelope_id;
    END;

CREATE TRIGGER IF NOT EXISTS trg_transactions_update_envelope_delete
    AFTER DELETE ON transactions
    WHEN OLD.envelope_id IS NOT NULL AND OLD.amount_cents < 0
    BEGIN
        UPDATE budget_envelopes
        SET spent_cents = MAX(0, spent_cents - ABS(OLD.amount_cents)),
            updated_at  = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
        WHERE id = OLD.envelope_id;
    END;

-- When a transaction is re-categorised (envelope_id changes), update both
-- the old and new envelopes.
CREATE TRIGGER IF NOT EXISTS trg_transactions_update_envelope_reassign
    AFTER UPDATE OF envelope_id ON transactions
    WHEN NEW.amount_cents < 0
    BEGIN
        -- subtract from old envelope
        UPDATE budget_envelopes
        SET spent_cents = MAX(0, spent_cents - ABS(OLD.amount_cents)),
            updated_at  = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
        WHERE id = OLD.envelope_id AND OLD.envelope_id IS NOT NULL;
        -- add to new envelope
        UPDATE budget_envelopes
        SET spent_cents = spent_cents + ABS(NEW.amount_cents),
            updated_at  = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
        WHERE id = NEW.envelope_id AND NEW.envelope_id IS NOT NULL;
    END;

-- ── updated_at triggers ──────────────────────────────────────────────────────
CREATE TRIGGER IF NOT EXISTS bank_accounts_updated_at
    AFTER UPDATE ON bank_accounts
    FOR EACH ROW WHEN OLD.updated_at = NEW.updated_at
    BEGIN
        UPDATE bank_accounts SET updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
        WHERE id = NEW.id;
    END;

CREATE TRIGGER IF NOT EXISTS transactions_updated_at
    AFTER UPDATE ON transactions
    FOR EACH ROW WHEN OLD.updated_at = NEW.updated_at
    BEGIN
        UPDATE transactions SET updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
        WHERE id = NEW.id;
    END;
