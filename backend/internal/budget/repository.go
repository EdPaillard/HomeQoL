package budget

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"homeqol/internal/db"
)

// Repository handles all SQL for the budget domain.
type Repository struct {
	db *db.DB
}

func NewRepository(database *db.DB) *Repository {
	return &Repository{db: database}
}

// ── Periods ───────────────────────────────────────────────────────────────────

func (r *Repository) GetPeriod(year, month int) (*BudgetPeriod, error) {
	const q = `SELECT id, year, month, is_closed, created_at FROM budget_periods WHERE year=? AND month=?`
	var p BudgetPeriod
	var isClosed int
	var createdAt string
	err := r.db.QueryRow(q, year, month).Scan(&p.ID, &p.Year, &p.Month, &isClosed, &createdAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get period: %w", err)
	}
	p.IsClosed = isClosed == 1
	p.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	p.Envelopes = []Envelope{}
	return &p, nil
}

func (r *Repository) CreatePeriod(id string, year, month int) (*BudgetPeriod, error) {
	_, err := r.db.Exec(
		`INSERT INTO budget_periods (id, year, month) VALUES (?, ?, ?)`,
		id, year, month,
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return nil, ErrPeriodExists
		}
		return nil, fmt.Errorf("create period: %w", err)
	}
	return r.GetPeriod(year, month)
}

// ── Templates ─────────────────────────────────────────────────────────────────

func (r *Repository) ListActiveTemplates() ([]EnvelopeTemplate, error) {
	const q = `
		SELECT id, name, budgeted_cents, color, icon, sort_order, is_active
		FROM envelope_templates WHERE is_active=1 ORDER BY sort_order ASC`
	rows, err := r.db.Query(q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var templates []EnvelopeTemplate
	for rows.Next() {
		var t EnvelopeTemplate
		var isActive int
		if err := rows.Scan(&t.ID, &t.Name, &t.BudgetedCents, &t.Color, &t.Icon, &t.SortOrder, &isActive); err != nil {
			return nil, err
		}
		t.IsActive = isActive == 1
		templates = append(templates, t)
	}
	return templates, rows.Err()
}

// ── Envelopes ─────────────────────────────────────────────────────────────────

func (r *Repository) ListEnvelopes(periodID string) ([]Envelope, error) {
	const q = `
		SELECT id, period_id, template_id, name, budgeted_cents, spent_cents,
		       color, icon, sort_order, created_at, updated_at
		FROM budget_envelopes WHERE period_id=? ORDER BY sort_order ASC`
	rows, err := r.db.Query(q, periodID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var envs []Envelope
	for rows.Next() {
		e, err := r.scanEnvelope(rows)
		if err != nil {
			return nil, err
		}
		envs = append(envs, *e)
	}
	return envs, rows.Err()
}

func (r *Repository) CreateEnvelope(id, periodID string, tpl EnvelopeTemplate) error {
	_, err := r.db.Exec(`
		INSERT INTO budget_envelopes (id, period_id, template_id, name, budgeted_cents, color, icon, sort_order)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		id, periodID, tpl.ID, tpl.Name, int64(tpl.BudgetedCents), tpl.Color, tpl.Icon, tpl.SortOrder,
	)
	return err
}

func (r *Repository) FindEnvelopeByTemplate(periodID, templateID string) (string, error) {
	var envelopeID string
	err := r.db.QueryRow(
		`SELECT id FROM budget_envelopes WHERE period_id=? AND template_id=?`,
		periodID, templateID,
	).Scan(&envelopeID)
	if err != nil {
		return "", ErrNotFound
	}
	return envelopeID, nil
}

// ── Transactions ──────────────────────────────────────────────────────────────

const txSelectCols = `
	id, account_id, envelope_id, external_id, label, clean_label,
	amount_cents, currency, transaction_date, category_rule, source, note,
	created_at, updated_at`

func (r *Repository) CreateTransaction(
	id string, req CreateTransactionRequest,
	envelopeID *string, ruleDesc *string, source string,
) (*Transaction, error) {
	cleanLabel := cleanLabel(req.Label)
	_, err := r.db.Exec(`
		INSERT INTO transactions
		  (id, account_id, envelope_id, label, clean_label, amount_cents,
		   transaction_date, category_rule, source, note)
		VALUES (?,?,?,?,?,?,?,?,?,?)`,
		id, req.AccountID, envelopeID, req.Label, cleanLabel,
		int64(req.AmountCents), req.Date, ruleDesc, source, req.Note,
	)
	if err != nil {
		return nil, fmt.Errorf("create transaction: %w", err)
	}
	return r.getTransaction(id)
}

func (r *Repository) CreateTransactionWithExternal(
	id string, req CreateTransactionRequest,
	externalID string, envelopeID *string, ruleDesc *string,
) (*Transaction, error) {
	cleanLabel := cleanLabel(req.Label)
	_, err := r.db.Exec(`
		INSERT INTO transactions
		  (id, account_id, envelope_id, external_id, label, clean_label,
		   amount_cents, transaction_date, category_rule, source, note)
		VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		id, req.AccountID, envelopeID, externalID,
		req.Label, cleanLabel, int64(req.AmountCents),
		req.Date, ruleDesc, "bank", req.Note,
	)
	if err != nil {
		return nil, fmt.Errorf("create bank transaction: %w", err)
	}
	return r.getTransaction(id)
}

func (r *Repository) TransactionExists(externalID string) (bool, error) {
	var n int
	err := r.db.QueryRow(
		`SELECT COUNT(*) FROM transactions WHERE external_id=?`, externalID,
	).Scan(&n)
	return n > 0, err
}

func (r *Repository) ListTransactions(envelopeID *string, limit, offset int) ([]Transaction, error) {
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	var q string
	var args []any
	if envelopeID != nil {
		q = `SELECT ` + txSelectCols + ` FROM transactions WHERE envelope_id=? ORDER BY transaction_date DESC LIMIT ? OFFSET ?`
		args = []any{*envelopeID, limit, offset}
	} else {
		q = `SELECT ` + txSelectCols + ` FROM transactions ORDER BY transaction_date DESC LIMIT ? OFFSET ?`
		args = []any{limit, offset}
	}
	rows, err := r.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var txs []Transaction
	for rows.Next() {
		tx, err := r.scanTransaction(rows)
		if err != nil {
			return nil, err
		}
		txs = append(txs, *tx)
	}
	return txs, rows.Err()
}

func (r *Repository) AssignEnvelope(transactionID, envelopeID string) (*Transaction, error) {
	_, err := r.db.Exec(
		`UPDATE transactions SET envelope_id=?, category_rule='manual' WHERE id=?`,
		envelopeID, transactionID,
	)
	if err != nil {
		return nil, fmt.Errorf("assign envelope: %w", err)
	}
	return r.getTransaction(transactionID)
}

func (r *Repository) getTransaction(id string) (*Transaction, error) {
	q := `SELECT ` + txSelectCols + ` FROM transactions WHERE id=?`
	tx, err := r.scanTransaction(r.db.QueryRow(q, id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return tx, nil
}

// ── Accounts ──────────────────────────────────────────────────────────────────

func (r *Repository) ListAccounts() ([]BankAccount, error) {
	const q = `
		SELECT id, external_id, label, type, balance_cents, currency,
		       iban, last_synced_at, created_at, updated_at
		FROM bank_accounts ORDER BY label ASC`
	rows, err := r.db.Query(q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var accounts []BankAccount
	for rows.Next() {
		a, err := r.scanAccount(rows)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, *a)
	}
	return accounts, rows.Err()
}

func (r *Repository) UpsertAccount(id, externalID, label, accountType string, balanceCents Cents) (*BankAccount, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := r.db.Exec(`
		INSERT INTO bank_accounts (id, external_id, label, type, balance_cents, last_synced_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(external_id) DO UPDATE SET
			label=excluded.label, balance_cents=excluded.balance_cents,
			last_synced_at=excluded.last_synced_at, updated_at=?`,
		id, externalID, label, accountType, int64(balanceCents), now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("upsert account: %w", err)
	}
	var acctID string
	r.db.QueryRow(`SELECT id FROM bank_accounts WHERE external_id=?`, externalID).Scan(&acctID)
	var a BankAccount
	err = r.db.QueryRow(`
		SELECT id, external_id, label, type, balance_cents, currency,
		       iban, last_synced_at, created_at, updated_at
		FROM bank_accounts WHERE id=?`, acctID).Scan(
		&a.ID, &a.ExternalID, &a.Label, &a.Type, &a.BalanceCents,
		&a.Currency, new(sql.NullString), new(sql.NullString), new(string), new(string),
	)
	if err != nil {
		return nil, err
	}
	a.BalanceEuros = a.BalanceCents.Euros()
	return &a, nil
}

// ── Rules ─────────────────────────────────────────────────────────────────────

func (r *Repository) ListActiveRules() ([]CategRule, error) {
	const q = `
		SELECT id, template_id, pattern, match_type, sort_order
		FROM categorisation_rules
		WHERE is_active=1 ORDER BY sort_order ASC`
	rows, err := r.db.Query(q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var rules []CategRule
	for rows.Next() {
		var ru CategRule
		if err := rows.Scan(&ru.ID, &ru.TemplateID, &ru.Pattern, &ru.MatchType, &ru.SortOrder); err != nil {
			return nil, err
		}
		rules = append(rules, ru)
	}
	return rules, rows.Err()
}

// ── Scanners ──────────────────────────────────────────────────────────────────

type scanner interface{ Scan(...any) error }

func (r *Repository) scanEnvelope(row scanner) (*Envelope, error) {
	var e Envelope
	var templateID sql.NullString
	var createdAt, updatedAt string
	err := row.Scan(
		&e.ID, &e.PeriodID, &templateID, &e.Name,
		&e.BudgetedCents, &e.SpentCents, &e.Color, &e.Icon,
		&e.SortOrder, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}
	if templateID.Valid {
		e.TemplateID = &templateID.String
	}
	e.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	e.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return &e, nil
}

func (r *Repository) scanTransaction(row scanner) (*Transaction, error) {
	var tx Transaction
	var accountID, envelopeID, externalID, categoryRule sql.NullString
	var createdAt, updatedAt string
	err := row.Scan(
		&tx.ID, &accountID, &envelopeID, &externalID,
		&tx.Label, &tx.CleanLabel, &tx.AmountCents, &tx.Currency,
		&tx.TransactionDate, &categoryRule, &tx.Source, &tx.Note,
		&createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}
	if accountID.Valid {
		tx.AccountID = &accountID.String
	}
	if envelopeID.Valid {
		tx.EnvelopeID = &envelopeID.String
	}
	if externalID.Valid {
		tx.ExternalID = &externalID.String
	}
	if categoryRule.Valid {
		tx.CategoryRule = &categoryRule.String
	}
	tx.AmountEuros = tx.AmountCents.Euros()
	tx.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	tx.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return &tx, nil
}

func (r *Repository) scanAccount(row scanner) (*BankAccount, error) {
	var a BankAccount
	var iban, lastSyncedAt sql.NullString
	var createdAt, updatedAt string
	err := row.Scan(
		&a.ID, &a.ExternalID, &a.Label, &a.Type, &a.BalanceCents,
		&a.Currency, &iban, &lastSyncedAt, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}
	if iban.Valid {
		a.IBAN = &iban.String
	}
	if lastSyncedAt.Valid {
		t, _ := time.Parse(time.RFC3339, lastSyncedAt.String)
		a.LastSyncedAt = &t
	}
	a.BalanceEuros = a.BalanceCents.Euros()
	a.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	a.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return &a, nil
}

// cleanLabel normalises a bank transaction label for display.
func cleanLabel(label string) string {
	label = strings.TrimSpace(label)
	// Remove common bank prefixes.
	prefixes := []string{"PAIEMENT CB ", "VIR SEPA ", "PRELEVEMENT ", "VIREMENT "}
	up := strings.ToUpper(label)
	for _, p := range prefixes {
		if strings.HasPrefix(up, p) {
			label = strings.TrimSpace(label[len(p):])
			break
		}
	}
	return label
}
