package habits

import (
	"context"
	"log/slog"
	"time"
)

// StreakJob recalculates streaks for all active habits once per day.
//
// Design decisions:
//
//  1. It runs as a single goroutine launched at server start, not as an
//     external cron job or separate process. This is the right approach for
//     a single-server personal project — zero infra overhead.
//
//  2. It fires at a fixed local time (default 00:05) by calculating the
//     duration until the next occurrence of that time, sleeping, running,
//     then repeating. This is more correct than a 24h ticker, which drifts.
//
//  3. It is timezone-aware. Streaks are about your day, not UTC midnight.
//     Pass your local timezone (e.g. time.LoadLocation("Europe/Paris")).
//
//  4. It respects context cancellation — when the server receives SIGTERM,
//     the graceful shutdown cancels the context and the job exits cleanly.
//
//  5. It can be triggered manually via TriggerNow() for immediate recalc
//     (useful after a backdated completion or for testing).

// StreakJob manages the scheduled streak recalculation.
type StreakJob struct {
	repo      *Repository
	logger    *slog.Logger
	location  *time.Location // timezone for the "run at midnight" calculation
	runAt     time.Duration  // duration from midnight, e.g. 5*time.Minute
	triggerCh chan struct{}  // manual trigger channel
}

// NewStreakJob creates a StreakJob.
//
// location: user's local timezone. Use time.LoadLocation("Europe/Paris") for France.
// runAt:    duration after midnight when the job runs, e.g. 5*time.Minute.
//
// Call Start(ctx) to launch the goroutine.
func NewStreakJob(repo *Repository, logger *slog.Logger, location *time.Location, runAt time.Duration) *StreakJob {
	if location == nil {
		location = time.Local
	}
	return &StreakJob{
		repo:      repo,
		logger:    logger,
		location:  location,
		runAt:     runAt,
		triggerCh: make(chan struct{}, 1), // buffered: won't block the caller
	}
}

// Start launches the streak job goroutine.
// It returns immediately; the goroutine runs until ctx is cancelled.
//
// Usage in main.go:
//
//	loc, _ := time.LoadLocation("Europe/Paris")
//	job := habits.NewStreakJob(repo, logger, loc, 5*time.Minute)
//	job.Start(ctx)
func (j *StreakJob) Start(ctx context.Context) {
	go j.loop(ctx)
}

// TriggerNow requests an immediate recalculation outside the normal schedule.
// Safe to call from any goroutine. Non-blocking: if a run is already queued,
// the extra trigger is dropped.
func (j *StreakJob) TriggerNow() {
	select {
	case j.triggerCh <- struct{}{}:
		j.logger.Info("streak job: manual trigger queued")
	default:
		// Already queued — don't block.
	}
}

// loop is the main goroutine. It sleeps until the next scheduled run,
// then recalculates all streaks.
func (j *StreakJob) loop(ctx context.Context) {
	j.logger.Info("streak job: started",
		slog.String("timezone", j.location.String()),
		slog.String("runs_at", formatDuration(j.runAt)+" after midnight"),
	)

	for {
		next := j.nextRunTime()
		wait := time.Until(next)

		j.logger.Debug("streak job: sleeping",
			slog.String("next_run", next.Format(time.RFC3339)),
			slog.String("wait", wait.Round(time.Second).String()),
		)

		select {
		case <-ctx.Done():
			j.logger.Info("streak job: stopped (context cancelled)")
			return

		case <-time.After(wait):
			j.run(ctx)

		case <-j.triggerCh:
			j.logger.Info("streak job: running (manual trigger)")
			j.run(ctx)
		}
	}
}

// run performs one full recalculation pass over all active habits.
func (j *StreakJob) run(ctx context.Context) {
	now := time.Now().In(j.location)
	j.logger.Info("streak job: recalculating", slog.String("local_time", now.Format(time.RFC3339)))
	start := time.Now()

	habits, err := j.repo.AllActiveIDs()
	if err != nil {
		j.logger.Error("streak job: failed to load habit IDs", slog.Any("error", err))
		return
	}

	updated := 0
	errors := 0

	for _, h := range habits {
		// Check for cancellation between habits so shutdown is prompt.
		select {
		case <-ctx.Done():
			j.logger.Info("streak job: cancelled mid-run")
			return
		default:
		}

		dates, err := j.repo.CompletionDates(h.ID)
		if err != nil {
			j.logger.Error("streak job: failed to load dates",
				slog.String("habit_id", h.ID),
				slog.Any("error", err),
			)
			errors++
			continue
		}

		// Calculate the streak using the pure functions from streak.go.
		var result StreakResult
		switch h.Frequency {
		case FrequencyWeekly:
			result = CalcWeeklyStreak(dates, now, h.TargetPerWeek)
		default:
			result = CalcDailyStreak(dates, now)
		}

		if err := j.repo.UpdateStreak(h.ID, result); err != nil {
			j.logger.Error("streak job: failed to write streak",
				slog.String("habit_id", h.ID),
				slog.Any("error", err),
			)
			errors++
			continue
		}
		updated++
	}

	j.logger.Info("streak job: done",
		slog.Int("habits_updated", updated),
		slog.Int("errors", errors),
		slog.String("duration", time.Since(start).String()),
	)
}

// nextRunTime calculates the next wall-clock time the job should fire.
// It always returns a time in the future.
func (j *StreakJob) nextRunTime() time.Time {
	now := time.Now().In(j.location)

	// Midnight of today in the local timezone.
	midnight := time.Date(
		now.Year(), now.Month(), now.Day(),
		0, 0, 0, 0,
		j.location,
	)

	// Target = midnight + runAt offset (e.g. 00:05).
	target := midnight.Add(j.runAt)

	// If target is in the past (i.e. it's already past 00:05 today),
	// schedule for tomorrow.
	if target.Before(now) || target.Equal(now) {
		target = target.AddDate(0, 0, 1)
	}

	return target
}

// formatDuration formats a duration as "HH:MM" for logging.
func formatDuration(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	return time.Date(0, 0, 0, h, m, 0, 0, time.UTC).Format("15:04")
}
