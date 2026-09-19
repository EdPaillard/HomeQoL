package goals

import (
	"fmt"
	"log/slog"

	"github.com/google/uuid"
)

// Service implements goals business logic.
type Service struct {
	repo   *Repository
	logger *slog.Logger
}

func NewService(repo *Repository, logger *slog.Logger) *Service {
	return &Service{repo: repo, logger: logger}
}

// Get returns a goal with its milestones and task progress populated.
func (s *Service) Get(id string) (*Goal, error) {
	g, err := s.repo.GetByID(id)
	if err != nil {
		return nil, err
	}
	return s.hydrate(g)
}

// List returns all top-level goals, each with their milestones hydrated.
func (s *Service) List() ([]Goal, error) {
	goals, err := s.repo.ListTopLevel()
	if err != nil {
		return nil, err
	}
	for i := range goals {
		h, err := s.hydrate(&goals[i])
		if err != nil {
			return nil, err
		}
		goals[i] = *h
	}
	return goals, nil
}

// Create validates and inserts a new goal.
func (s *Service) Create(req CreateRequest) (*Goal, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	// If a parent is specified, verify it exists.
	if req.ParentID != nil {
		if _, err := s.repo.GetByID(*req.ParentID); err != nil {
			return nil, fmt.Errorf("parent goal not found")
		}
	}
	id := uuid.New().String()
	s.logger.Info("creating goal", slog.String("id", id), slog.String("title", req.Title))
	g, err := s.repo.Create(id, req)
	if err != nil {
		return nil, err
	}
	return s.hydrate(g)
}

// Update applies a partial update and enforces completion rules.
func (s *Service) Update(id string, req UpdateRequest) (*Goal, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	g, err := s.repo.Update(id, req)
	if err != nil {
		return nil, err
	}
	return s.hydrate(g)
}

// Delete soft-deletes a goal. Refuses if it has child milestones.
func (s *Service) Delete(id string) error {
	n, err := s.repo.ChildCount(id)
	if err != nil {
		return err
	}
	if n > 0 {
		return ErrHasChildren
	}
	return s.repo.Delete(id)
}

// hydrate populates computed fields: milestones, task counts, progress %.
func (s *Service) hydrate(g *Goal) (*Goal, error) {
	milestones, err := s.repo.ListMilestones(g.ID)
	if err != nil {
		return nil, err
	}
	// Hydrate each milestone's own progress.
	for i := range milestones {
		total, done, err := s.repo.TaskCounts(milestones[i].ID)
		if err != nil {
			return nil, err
		}
		milestones[i].TaskCount = total
		milestones[i].DoneCount = done
		if total > 0 {
			milestones[i].ProgressPct = float64(done) / float64(total) * 100
		}
	}
	g.Milestones = milestones

	total, done, err := s.repo.TaskCounts(g.ID)
	if err != nil {
		return nil, err
	}
	g.TaskCount = total
	g.DoneCount = done
	if total > 0 {
		g.ProgressPct = float64(done) / float64(total) * 100
	}
	return g, nil
}
