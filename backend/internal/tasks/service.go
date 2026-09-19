package tasks

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
)

// Service implements the business logic for the tasks domain.
// It sits between the HTTP handler (which knows about requests/responses)
// and the repository (which knows about SQL).
//
// Rules enforced here:
//   - IDs are generated here (UUID v4), never by the DB or the client
//   - Completed tasks cannot be moved back to in_progress directly
//   - Cancelled tasks can only be re-opened to todo
//   - Tag IDs are validated before being applied
type Service struct {
	repo   *Repository
	logger *slog.Logger
}

// NewService creates a Service.
func NewService(repo *Repository, logger *slog.Logger) *Service {
	return &Service{repo: repo, logger: logger}
}

// Get returns a single task by ID.
func (s *Service) Get(id string) (*Task, error) {
	task, err := s.repo.GetByID(id)
	if err != nil {
		return nil, err // ErrNotFound propagates as-is
	}
	return task, nil
}

// List returns tasks matching the filter.
func (s *Service) List(f ListFilter) ([]Task, error) {
	tasks, err := s.repo.List(f)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	return tasks, nil
}

// Create validates the request and creates a new task.
func (s *Service) Create(req CreateRequest) (*Task, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	id := uuid.New().String()

	s.logger.Info("creating task",
		slog.String("id", id),
		slog.String("title", req.Title),
		slog.Int("priority", int(req.Priority)),
	)

	task, err := s.repo.Create(id, req)
	if err != nil {
		if errors.Is(err, ErrTagNotFound) {
			return nil, err
		}
		return nil, fmt.Errorf("create task: %w", err)
	}
	return task, nil
}

// Update validates and applies a partial update.
// Business rules around status transitions are enforced here.
func (s *Service) Update(id string, req UpdateRequest) (*Task, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	// Enforce status transition rules before hitting the DB.
	if req.Status != nil {
		current, err := s.repo.GetByID(id)
		if err != nil {
			return nil, err
		}
		if err := validateTransition(current.Status, *req.Status); err != nil {
			return nil, err
		}
	}

	task, err := s.repo.Update(id, req)
	if err != nil {
		return nil, err
	}

	s.logger.Info("updated task", slog.String("id", id))
	return task, nil
}

// Complete is a convenience method — marks a task done without a full UpdateRequest.
func (s *Service) Complete(id string) (*Task, error) {
	status := StatusDone
	return s.Update(id, UpdateRequest{Status: &status})
}

// Delete soft-deletes a task.
func (s *Service) Delete(id string) error {
	if err := s.repo.Delete(id); err != nil {
		return err
	}
	s.logger.Info("deleted task", slog.String("id", id))
	return nil
}

// ── Status transition rules ───────────────────────────────────────────────────
// A state machine encoded as a simple allowlist.
// This is the kind of rule that belongs in the service, not in a DB constraint,
// because it requires knowing the *previous* state.
//
//	todo → in_progress, done, cancelled        ✓
//	in_progress → todo, done, cancelled        ✓
//	done → todo                                ✓ (re-open)
//	done → in_progress                         ✗ (must go via todo first)
//	cancelled → todo                           ✓ (re-open)
//	cancelled → anything else                  ✗

var allowedTransitions = map[Status][]Status{
	StatusTodo:       {StatusInProgress, StatusDone, StatusCancelled},
	StatusInProgress: {StatusTodo, StatusDone, StatusCancelled},
	StatusDone:       {StatusTodo},
	StatusCancelled:  {StatusTodo},
}

func validateTransition(from, to Status) error {
	if from == to {
		return nil // no-op transition is always fine
	}
	for _, allowed := range allowedTransitions[from] {
		if allowed == to {
			return nil
		}
	}
	return fmt.Errorf(
		"cannot transition task from %q to %q",
		string(from), string(to),
	)
}
