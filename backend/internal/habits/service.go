package habits

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

// Service is the habits business-logic layer.
// It owns the StreakJob goroutine and exposes methods the handler calls.
type Service struct {
	repo      *Repository
	logger    *slog.Logger
	streakJob *StreakJob
	now       func() time.Time // injectable for tests
}

// NewService creates a Service and configures the streak job.
//
// loc is the user's local timezone — pass time.LoadLocation("Europe/Paris").
// The streak job is wired here but not started; call Start(ctx) separately
// so the caller controls the lifecycle.
func NewService(repo *Repository, logger *slog.Logger, loc *time.Location) *Service {
	job := NewStreakJob(repo, logger, loc, 5*time.Minute) // runs at 00:05 local
	return &Service{
		repo:      repo,
		logger:    logger,
		streakJob: job,
		now:       time.Now,
	}
}

// Start launches the background streak goroutine.
// Must be called once after NewService; the goroutine exits when ctx is cancelled.
func (s *Service) Start(ctx context.Context) {
	s.streakJob.Start(ctx)
}

// TriggerStreakRecalc requests an immediate streak recalculation.
// Useful after a backdated completion — call it and the update arrives within
// seconds rather than waiting until 00:05.
func (s *Service) TriggerStreakRecalc() {
	s.streakJob.TriggerNow()
}

// ── CRUD ──────────────────────────────────────────────────────────────────────

func (s *Service) Get(id string) (*Habit, error) {
	return s.repo.GetByID(id)
}

func (s *Service) List() ([]Habit, error) {
	today := s.now().Format("2006-01-02")
	return s.repo.ListActive(today)
}

func (s *Service) Create(req CreateRequest) (*Habit, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	id := uuid.New().String()
	s.logger.Info("creating habit", slog.String("id", id), slog.String("title", req.Title))
	return s.repo.Create(id, req)
}

func (s *Service) Update(id string, req UpdateRequest) (*Habit, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	if _, err := s.repo.GetByID(id); err != nil {
		return nil, err
	}
	return s.repo.Update(id, req)
}

func (s *Service) Delete(id string) error {
	return s.repo.Delete(id)
}

// ── Completions ───────────────────────────────────────────────────────────────

// Complete logs a completion for today (or a specified date).
// After a successful log, it triggers a streak recalc so the UI updates
// immediately without waiting for the nightly job.
func (s *Service) Complete(habitID string, req CompleteRequest) (*Completion, error) {
	if err := req.Validate(s.now()); err != nil {
		return nil, err
	}

	// Confirm the habit exists.
	if _, err := s.repo.GetByID(habitID); err != nil {
		return nil, err
	}

	id := uuid.New().String()
	completion, err := s.repo.AddCompletion(id, habitID, req.Date, req.Note)
	if err != nil {
		if errors.Is(err, ErrAlreadyDone) {
			return nil, ErrAlreadyDone
		}
		return nil, fmt.Errorf("add completion: %w", err)
	}

	// Trigger an async streak recalc — the cron will also run tonight,
	// but this makes the streak update visible within seconds.
	s.streakJob.TriggerNow()

	s.logger.Info("habit completed",
		slog.String("habit_id", habitID),
		slog.String("date", req.Date),
	)
	return completion, nil
}

// Uncomplete removes a completion (the user "un-ticked" a habit).
func (s *Service) Uncomplete(habitID, completionID string) error {
	if err := s.repo.DeleteCompletion(completionID, habitID); err != nil {
		return err
	}
	s.streakJob.TriggerNow()
	return nil
}
