package budget

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

// Service implements budget business logic.
//
// Key responsibilities:
//   - Creating periods and seeding them with envelope templates
//   - Categorising imported transactions automatically
//   - Providing summary views (period totals, per-envelope spend)
type Service struct {
	repo   *Repository
	logger *slog.Logger
	now    func() time.Time
}

func NewService(repo *Repository, logger *slog.Logger) *Service {
	return &Service{repo: repo, logger: logger, now: time.Now}
}

// ── Periods ───────────────────────────────────────────────────────────────────

// GetOrCreateCurrentPeriod returns the budget period for the current month,
// creating it (and seeding envelopes from templates) if it doesn't exist yet.
// This is called on app startup and by the daily cron so you never have to
// manually create a new period.
func (s *Service) GetOrCreateCurrentPeriod() (*BudgetPeriod, error) {
	now := s.now()
	return s.GetOrCreatePeriod(now.Year(), int(now.Month()))
}

// GetOrCreatePeriod returns or creates the period for a specific year/month.
func (s *Service) GetOrCreatePeriod(year, month int) (*BudgetPeriod, error) {
	existing, err := s.repo.GetPeriod(year, month)
	if err == nil {
		return existing, nil // already exists
	}

	// Create a new period.
	periodID := uuid.New().String()
	_, err = s.repo.CreatePeriod(periodID, year, month)
	if err != nil {
		return nil, fmt.Errorf("create period %d/%02d: %w", year, month, err)
	}

	// Seed envelopes from templates.
	templates, err := s.repo.ListActiveTemplates()
	if err != nil {
		return nil, fmt.Errorf("load templates: %w", err)
	}

	for _, tpl := range templates {
		envID := uuid.New().String()
		if err := s.repo.CreateEnvelope(envID, periodID, tpl); err != nil {
			return nil, fmt.Errorf("seed envelope %q: %w", tpl.Name, err)
		}
	}

	s.logger.Info("budget period created",
		slog.Int("year", year),
		slog.Int("month", month),
		slog.Int("envelopes_seeded", len(templates)),
	)

	return s.repo.GetPeriod(year, month)
}

// GetPeriodWithEnvelopes returns a period with full envelope data and summary.
func (s *Service) GetPeriodWithEnvelopes(year, month int) (*BudgetPeriod, error) {
	period, err := s.repo.GetPeriod(year, month)
	if err != nil {
		return nil, err
	}

	envelopes, err := s.repo.ListEnvelopes(period.ID)
	if err != nil {
		return nil, err
	}

	// Fill computed fields and build summary.
	var totalBudgeted, totalSpent Cents
	for i := range envelopes {
		envelopes[i].FillComputed()
		totalBudgeted += envelopes[i].BudgetedCents
		totalSpent += envelopes[i].SpentCents
	}

	period.Envelopes = envelopes
	period.Summary = PeriodSummary{
		TotalBudgetedCents: totalBudgeted,
		TotalSpentCents:    totalSpent,
		TotalBudgetedEuros: totalBudgeted.Euros(),
		TotalSpentEuros:    totalSpent.Euros(),
		RemainingCents:     totalBudgeted - totalSpent,
		RemainingEuros:     (totalBudgeted - totalSpent).Euros(),
	}
	return period, nil
}

// ── Transactions ──────────────────────────────────────────────────────────────

// CreateManualTransaction records a manually entered transaction and
// attempts to auto-categorise it using the active rules.
func (s *Service) CreateManualTransaction(req CreateTransactionRequest) (*Transaction, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	envelopeID, ruleDesc, err := s.autoAssignEnvelope(req.Label, req.Date)
	if err != nil {
		return nil, err
	}

	// Let the caller override the auto-assignment.
	if req.EnvelopeID != nil {
		envelopeID = req.EnvelopeID
		ruleDesc = "manual"
	}

	id := uuid.New().String()
	return s.repo.CreateTransaction(id, req, envelopeID, &ruleDesc, "manual")
}

// ImportBankTransaction is called by the Powens sync job.
// It deduplicates by external_id and auto-categorises.
func (s *Service) ImportBankTransaction(
	externalID, accountID, label string,
	amountCents Cents,
	date string,
) (*Transaction, error) {
	// Idempotent: skip if already imported.
	if exists, _ := s.repo.TransactionExists(externalID); exists {
		return nil, nil
	}

	envelopeID, ruleDesc, err := s.autoAssignEnvelope(label, date)
	if err != nil {
		return nil, err
	}

	req := CreateTransactionRequest{
		AccountID:   &accountID,
		Label:       label,
		AmountCents: amountCents,
		Date:        date,
	}
	id := uuid.New().String()
	tx, err := s.repo.CreateTransactionWithExternal(id, req, externalID, envelopeID, &ruleDesc)
	if err != nil {
		return nil, fmt.Errorf("import transaction %s: %w", externalID, err)
	}

	s.logger.Debug("transaction imported",
		slog.String("external_id", externalID),
		slog.String("label", label),
		slog.String("rule", ruleDesc),
	)
	return tx, nil
}

// AssignEnvelope manually re-categorises a transaction.
func (s *Service) AssignEnvelope(transactionID string, req AssignEnvelopeRequest) (*Transaction, error) {
	return s.repo.AssignEnvelope(transactionID, req.EnvelopeID)
}

// ListTransactions returns transactions for an envelope or period.
func (s *Service) ListTransactions(envelopeID *string, limit, offset int) ([]Transaction, error) {
	return s.repo.ListTransactions(envelopeID, limit, offset)
}

// ── Accounts ──────────────────────────────────────────────────────────────────

func (s *Service) ListAccounts() ([]BankAccount, error) {
	return s.repo.ListAccounts()
}

// UpsertAccount creates or updates a bank account from a Powens sync.
// "Upsert" = update if external_id exists, insert otherwise.
func (s *Service) UpsertAccount(externalID, label, accountType string, balanceCents Cents) (*BankAccount, error) {
	id := uuid.New().String()
	return s.repo.UpsertAccount(id, externalID, label, accountType, balanceCents)
}

// ── Categorisation ────────────────────────────────────────────────────────────

// autoAssignEnvelope loads rules, runs the categoriser, and resolves the
// template match to the correct envelope for the transaction's period.
func (s *Service) autoAssignEnvelope(label, date string) (*string, string, error) {
	rules, err := s.repo.ListActiveRules()
	if err != nil {
		return nil, "", fmt.Errorf("load categorisation rules: %w", err)
	}

	cat, err := NewCategoriser(rules)
	if err != nil {
		return nil, "", fmt.Errorf("build categoriser: %w", err)
	}

	templateID, ruleDesc := cat.Categorise(label)
	if templateID == "" {
		return nil, "unmatched", nil // no rule matched — envelope unset
	}

	// Resolve which period this date belongs to.
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return nil, "", err
	}
	period, err := s.GetOrCreatePeriod(t.Year(), int(t.Month()))
	if err != nil {
		return nil, "", err
	}

	envelopeID, err := s.repo.FindEnvelopeByTemplate(period.ID, templateID)
	if err != nil {
		// Template matched but no envelope in this period — return unassigned.
		s.logger.Warn("no envelope found for template",
			slog.String("template_id", templateID),
			slog.String("period", fmt.Sprintf("%d/%02d", period.Year, period.Month)),
		)
		return nil, ruleDesc, nil
	}

	return &envelopeID, ruleDesc, nil
}
