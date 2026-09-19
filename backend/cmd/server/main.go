package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"homeqol/config"
	"homeqol/internal/db"
)

func main() {
	// ── Config ───────────────────────────────────────────────────────────────
	cfg := config.Load()

	// ── Logger ───────────────────────────────────────────────────────────────
	// Development: human-readable text. Production: structured JSON for Loki.
	var logHandler slog.Handler
	if cfg.IsDevelopment() {
		logHandler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		})
	} else {
		logHandler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		})
	}
	logger := slog.New(logHandler)
	slog.SetDefault(logger)

	// ── Database ──────────────────────────────────────────────────────────────
	database, err := db.Open(cfg.DBPath, logger)
	if err != nil {
		logger.Error("failed to open database", slog.Any("error", err))
		os.Exit(1)
	}
	defer database.Close()
	logger.Info("database ready", slog.String("path", cfg.DBPath))

	// ── Application context ───────────────────────────────────────────────────
	// Cancelled on shutdown, which stops all background goroutines cleanly
	// before the HTTP server finishes draining in-flight requests.
	appCtx, cancelApp := context.WithCancel(context.Background())
	defer cancelApp()

	// ── Router + services ─────────────────────────────────────────────────────
	router, svcs := newRouter(cfg, logger, database)

	// ── Background goroutines ─────────────────────────────────────────────────

	// Habits streak job — nightly recalculation at 00:05 local time.
	svcs.habits.Start(appCtx)
	logger.Info("habit streak job started", slog.String("timezone", cfg.TZ))

	// Home — MQTT subscriber for real-time Tydom data.
	if cfg.MQTTEnabled() {
		if err := svcs.home.StartMQTTSubscriber(appCtx, cfg.MQTTBrokerURL); err != nil {
			logger.Warn("MQTT subscriber failed to start",
				slog.Any("error", err),
				slog.String("broker", cfg.MQTTBrokerURL),
			)
		} else {
			logger.Info("MQTT subscriber started", slog.String("broker", cfg.MQTTBrokerURL))
		}
	} else {
		logger.Info("MQTT disabled — set MQTT_BROKER_URL to enable Tydom readings")
	}

	// Home — Linky daily cron (fetches yesterday's kWh from Conso API at 08:34).
	if cfg.LinkyEnabled() {
		svcs.home.StartLinkyJob(appCtx, cfg.ConsoAPIToken, cfg.PRM, cfg.Location())
		logger.Info("Linky cron started")
	} else {
		logger.Info("Linky disabled — set CONSO_API_TOKEN to enable energy tracking")
	}

	// Budget — ensure current month's envelope period exists on startup.
	if _, err := svcs.budget.GetOrCreateCurrentPeriod(); err != nil {
		logger.Warn("failed to initialise current budget period", slog.Any("error", err))
	}

	// ── HTTP server ───────────────────────────────────────────────────────────
	server := &http.Server{
		Addr:              cfg.Addr(),
		Handler:           router,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("server starting",
			slog.String("addr", cfg.Addr()),
			slog.String("env", cfg.Env),
			slog.String("service", cfg.ServiceName),
			slog.String("version", cfg.ServiceVersion),
		)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server error", slog.Any("error", err))
			os.Exit(1)
		}
	}()

	// ── Graceful shutdown ─────────────────────────────────────────────────────
	shutdownCh := make(chan os.Signal, 1)
	signal.Notify(shutdownCh, syscall.SIGINT, syscall.SIGTERM)

	sig := <-shutdownCh
	logger.Info("shutdown signal received", slog.String("signal", sig.String()))

	// Cancel app context first — all background goroutines exit.
	cancelApp()

	// Give in-flight HTTP requests 15 s to finish.
	drainCtx, drainCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer drainCancel()

	if err := server.Shutdown(drainCtx); err != nil {
		logger.Error("graceful shutdown failed", slog.Any("error", err))
		os.Exit(1)
	}

	logger.Info("server stopped cleanly")
}
