// Package budget implements envelope budgeting with bank transaction sync.
package budget

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// ── Money ─────────────────────────────────────────────────────────────────────

// Cents represents a monetary value in euro cents.
// Always use this — never float64 for money.
type Cents int64

func (c Cents) Euros() float64 { return float64(c) / 100 }
func (c Cents) String() string { return fmt.Sprintf("%.2f €", c.Euros()) }

// ── Domain types ──────────────────────────────────────────────────────────────

type BankAccount struct {
	ID           string     `json:"id"`
	ExternalID   string     `json:"external_id"`
	Label        string     `json:"label"`
	Type         string     `json:"type"`
	BalanceCents Cents      `json:"balance_cents"`
	BalanceEuros float64    `json:"balance_euros"`
	Currency     string     `json:"currency"`
	IBAN         *string    `json:"iban,omitempty"`
	LastSyncedAt *time.Time `json:"last_synced_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

type BudgetPeriod struct {
	ID        string        `json:"id"`
	Year      int           `json:"year"`
	Month     int           `json:"month"`
	IsClosed  bool          `json:"is_closed"`
	Envelopes []Envelope    `json:"envelopes"`
	Summary   PeriodSummary `json:"summary"`
	CreatedAt time.Time     `json:"created_at"`
}

type PeriodSummary struct {
	TotalBudgetedCents Cents   `json:"total_budgeted_cents"`
	TotalSpentCents    Cents   `json:"total_spent_cents"`
	TotalBudgetedEuros float64 `json:"total_budgeted_euros"`
	TotalSpentEuros    float64 `json:"total_spent_euros"`
	RemainingCents     Cents   `json:"remaining_cents"`
	RemainingEuros     float64 `json:"remaining_euros"`
}

type Envelope struct {
	ID             string    `json:"id"`
	PeriodID       string    `json:"period_id"`
	TemplateID     *string   `json:"template_id,omitempty"`
	Name           string    `json:"name"`
	BudgetedCents  Cents     `json:"budgeted_cents"`
	SpentCents     Cents     `json:"spent_cents"`
	BudgetedEuros  float64   `json:"budgeted_euros"`
	SpentEuros     float64   `json:"spent_euros"`
	RemainingCents Cents     `json:"remaining_cents"`
	PercentUsed    float64   `json:"percent_used"`
	Color          string    `json:"color"`
	Icon           string    `json:"icon"`
	SortOrder      int       `json:"sort_order"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// FillComputed sets euro and percentage fields from the stored cent values.
func (e *Envelope) FillComputed() {
	e.BudgetedEuros = e.BudgetedCents.Euros()
	e.SpentEuros = e.SpentCents.Euros()
	e.RemainingCents = e.BudgetedCents - e.SpentCents
	if e.BudgetedCents > 0 {
		e.PercentUsed = float64(e.SpentCents) / float64(e.BudgetedCents) * 100
	}
}

type Transaction struct {
	ID              string    `json:"id"`
	AccountID       *string   `json:"account_id,omitempty"`
	EnvelopeID      *string   `json:"envelope_id,omitempty"`
	ExternalID      *string   `json:"external_id,omitempty"`
	Label           string    `json:"label"`
	CleanLabel      string    `json:"clean_label"`
	AmountCents     Cents     `json:"amount_cents"`
	AmountEuros     float64   `json:"amount_euros"`
	Currency        string    `json:"currency"`
	TransactionDate string    `json:"transaction_date"`
	CategoryRule    *string   `json:"category_rule,omitempty"`
	Source          string    `json:"source"`
	Note            string    `json:"note"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type EnvelopeTemplate struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	BudgetedCents Cents  `json:"budgeted_cents"`
	Color         string `json:"color"`
	Icon          string `json:"icon"`
	SortOrder     int    `json:"sort_order"`
	IsActive      bool   `json:"is_active"`
}

type CategRule struct {
	ID         string `json:"id"`
	TemplateID string `json:"template_id"`
	Pattern    string `json:"pattern"`
	MatchType  string `json:"match_type"` // contains | starts_with | regex
	SortOrder  int    `json:"sort_order"`
}

// ── Categorisation engine ─────────────────────────────────────────────────────

// Categoriser holds compiled rules and matches transaction labels to template IDs.
// It is built once at startup and reused for every transaction.
type Categoriser struct {
	rules []compiledRule
}

type compiledRule struct {
	CategRule
	re *regexp.Regexp // non-nil only for match_type=regex
}

// NewCategoriser compiles a slice of rules into a Categoriser.
// Rules must be sorted by sort_order ASC before being passed in —
// the repository does this via ORDER BY.
func NewCategoriser(rules []CategRule) (*Categoriser, error) {
	compiled := make([]compiledRule, 0, len(rules))
	for _, r := range rules {
		cr := compiledRule{CategRule: r}
		if r.MatchType == "regex" {
			re, err := regexp.Compile("(?i)" + r.Pattern) // case-insensitive
			if err != nil {
				return nil, fmt.Errorf("invalid regex rule %s %q: %w", r.ID, r.Pattern, err)
			}
			cr.re = re
		}
		compiled = append(compiled, cr)
	}
	return &Categoriser{rules: compiled}, nil
}

// Categorise returns the templateID and matching rule description for a label.
// Returns ("", "") if no rule matches.
func (c *Categoriser) Categorise(label string) (templateID string, ruleDesc string) {
	upper := strings.ToUpper(label)
	for _, r := range c.rules {
		patUpper := strings.ToUpper(r.Pattern)
		var matched bool
		switch r.MatchType {
		case "contains":
			matched = strings.Contains(upper, patUpper)
		case "starts_with":
			matched = strings.HasPrefix(upper, patUpper)
		case "regex":
			matched = r.re != nil && r.re.MatchString(label)
		}
		if matched {
			return r.TemplateID, fmt.Sprintf("rule:%s(%s)", r.MatchType, r.Pattern)
		}
	}
	return "", ""
}

// ── Request / response DTOs ───────────────────────────────────────────────────

type CreatePeriodRequest struct {
	Year  int `json:"year"`
	Month int `json:"month"`
}

func (r *CreatePeriodRequest) Validate() error {
	if r.Year < 2020 || r.Year > 2100 {
		return errors.New("year must be between 2020 and 2100")
	}
	if r.Month < 1 || r.Month > 12 {
		return errors.New("month must be between 1 and 12")
	}
	return nil
}

type CreateTransactionRequest struct {
	AccountID   *string `json:"account_id"`
	EnvelopeID  *string `json:"envelope_id"`
	Label       string  `json:"label"`
	AmountCents Cents   `json:"amount_cents"` // negative=expense, positive=income
	Date        string  `json:"date"`         // "YYYY-MM-DD"
	Note        string  `json:"note"`
}

func (r *CreateTransactionRequest) Validate() error {
	r.Label = strings.TrimSpace(r.Label)
	if r.Label == "" {
		return errors.New("label is required")
	}
	if _, err := time.Parse("2006-01-02", r.Date); err != nil {
		return errors.New("date must be YYYY-MM-DD")
	}
	return nil
}

type AssignEnvelopeRequest struct {
	EnvelopeID string `json:"envelope_id"`
}

// ── Sentinel errors ───────────────────────────────────────────────────────────

var (
	ErrNotFound     = errors.New("not found")
	ErrPeriodExists = errors.New("budget period already exists for this month")
	ErrPeriodClosed = errors.New("budget period is closed")
)
