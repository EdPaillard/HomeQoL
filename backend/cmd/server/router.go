package main

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"homeqol/config"
	"homeqol/internal/budget"
	"homeqol/internal/db"
	"homeqol/internal/goals"
	"homeqol/internal/habits"
	"homeqol/internal/health"
	"homeqol/internal/home"
	"homeqol/internal/middleware"
	"homeqol/internal/tasks"
)

// services bundles every initialised service so main.go can start background
// goroutines (streak job, MQTT subscriber, Linky cron) after the HTTP server
// is already listening.
type services struct {
	habits *habits.Service
	home   *home.Service
	budget *budget.Service
}

// newRouter builds the fully configured Chi router and returns both the handler
// and the initialised services (so main can call their Start methods).
//
// Middleware stack (outermost → innermost):
//  1. Recoverer  — catch any panics, return 500
//  2. RequestID  — stamp X-Request-Id on every request
//  3. Logger     — one structured slog line per request
//  4. RealIP     — honour X-Forwarded-For from a reverse proxy
//  5. Compress   — gzip responses above 1 KB
//
// Routes under /api/v1:
//
//	/tasks    — CRUD, tagging, status transitions
//	/habits   — CRUD, completions, streak reads
//	/goals    — CRUD, milestone nesting, task progress
//	/budget   — periods, envelopes, transactions, accounts
//	/home     — dashboard, temperature/energy history, alerts
func newRouter(cfg config.Config, logger *slog.Logger, database *db.DB) (http.Handler, *services) {
	r := chi.NewRouter()

	// ── Global middleware ────────────────────────────────────────────────────
	r.Use(middleware.Recoverer(logger))
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(middleware.Logger(logger))
	r.Use(chimiddleware.Compress(5))

	// ── Health ───────────────────────────────────────────────────────────────
	h := health.New(cfg.ServiceName, cfg.ServiceVersion)
	h.AddCheck("database", database.Ping)
	r.Get("/health", h.LivenessHandler)
	r.Get("/ready", h.ReadinessHandler)

	// ── Build services ───────────────────────────────────────────────────────
	// Each domain: repository → service → handler → mount.

	// Tasks
	tasksSvc := tasks.NewService(tasks.NewRepository(database), logger)

	// Goals
	goalsSvc := goals.NewService(goals.NewRepository(database), logger)

	// Habits — timezone is read from TZ env var (falls back to time.Local).
	// The streak job is started by main after the HTTP server is listening.
	loc := cfg.Location()
	habitsSvc := habits.NewService(habits.NewRepository(database), logger, loc)

	// Budget — auto-creates the current month's period on first /current hit.
	budgetSvc := budget.NewService(budget.NewRepository(database), logger)

	// Home monitor — MQTT and Linky are started by main.
	homeSvc := home.NewService(home.NewRepository(database), logger, home.DefaultThresholds)

	// ── Mount routes ─────────────────────────────────────────────────────────
	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"message":"personal-os API v1"}`))
		})

		r.Mount("/tasks", tasks.NewRouter(tasksSvc))
		r.Mount("/habits", habits.NewRouter(habitsSvc))
		r.Mount("/goals", goals.NewRouter(goalsSvc))
		r.Mount("/budget", budget.NewRouter(budgetSvc))
		r.Mount("/home", home.NewRouter(homeSvc))
	})

	// ── 404 fallback ─────────────────────────────────────────────────────────
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"not found"}`))
	})

	return r, &services{
		habits: habitsSvc,
		home:   homeSvc,
		budget: budgetSvc,
	}
}
