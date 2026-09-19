package home

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// LinkyJob fetches yesterday's energy consumption from Enedis via Conso API
// once per day at 08:34 local time.
//
// Conso API (https://conso.boris.sh) acts as a proxy to the Enedis Data Connect
// API. It requires a Bearer token obtained by authorising access once at
// https://conso.boris.sh via your Enedis account.
//
// Data availability: Linky transmits data once per day, typically available
// by 08:00. We wait until 08:34 to be safe.
type LinkyJob struct {
	svc        *Service
	logger     *slog.Logger
	token      string
	location   *time.Location
	httpClient *http.Client
	prm        string // Conso API parameter, not documented but required in practice
}

const consoAPIBase = "https://conso.boris.sh/api"

func NewLinkyJob(svc *Service, logger *slog.Logger, token, prm string, loc *time.Location) *LinkyJob {
	if loc == nil {
		loc = time.Local
	}
	return &LinkyJob{
		svc:        svc,
		logger:     logger,
		token:      token,
		location:   loc,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		prm:        prm,
	}
}

// Start launches the daily Linky fetch goroutine.
func (j *LinkyJob) Start(ctx context.Context) {
	go j.loop(ctx)
}

func (j *LinkyJob) loop(ctx context.Context) {
	j.logger.Info("Linky job: started",
		slog.String("timezone", j.location.String()),
	)

	for {
		next := j.nextRunTime()
		j.logger.Debug("Linky job: sleeping",
			slog.String("next_run", next.Format(time.RFC3339)),
		)

		select {
		case <-ctx.Done():
			j.logger.Info("Linky job: stopped")
			return
		case <-time.After(time.Until(next)):
			j.fetch(ctx)
		}
	}
}

// fetch fetches yesterday's daily total and the 24 hourly slots if available.
func (j *LinkyJob) fetch(ctx context.Context) {
	now := time.Now().In(j.location)
	yesterday := now.AddDate(0, 0, -1).Format("2006-01-02")
	nowFmt := now.Format("2006-01-02")

	j.logger.Info("Linky job: fetching", slog.String("date", yesterday))

	// ── Daily total ────────────────────────────────────────────────────────
	daily, err := j.fetchDailyConsumption(ctx, yesterday, nowFmt)
	if err != nil {
		j.logger.Error("Linky job: daily fetch failed", slog.Any("error", err))
	} else {
		for _, reading := range daily {
			if _, err := j.svc.IngestEnergy(IngestEnergyRequest{
				Date:        reading.Date,
				KWh:         reading.Value,
				Granularity: "daily",
			}); err != nil {
				j.logger.Warn("Linky job: ingest daily failed",
					slog.String("date", reading.Date), slog.Any("error", err))
			}
		}
	}

	// ── Hourly slots ───────────────────────────────────────────────────────
	// Not all meters/subscriptions have hourly data — skip silently if unavailable.
	hourly, err := j.fetchHourlyConsumption(ctx, yesterday, nowFmt)
	if err != nil {
		j.logger.Debug("Linky job: hourly data unavailable", slog.Any("error", err))
	} else {
		for _, reading := range hourly {
			hour := reading.Hour
			if _, err := j.svc.IngestEnergy(IngestEnergyRequest{
				Date:        reading.Date,
				Hour:        &hour,
				KWh:         reading.Value,
				Granularity: "hourly",
			}); err != nil {
				j.logger.Warn("Linky job: ingest hourly failed", slog.Any("error", err))
			}
		}
	}
}

// ── Conso API client ──────────────────────────────────────────────────────────

type consoReading struct {
	Date  string
	Hour  int
	Value float64
}

// consoResponse is the shape returned by the Conso API v5 endpoints.
type consoResponse struct {
	IntervalReading []struct {
		Value     string `json:"value"`
		DateDebut string `json:"date"` // "YYYY-MM-DD" or "YYYY-MM-DDTHH:mm:ss"
	} `json:"interval_reading"`
	ReadingType struct {
		Unit string `json:"unit"` // "Wh"
	} `json:"reading_type"`
}

func (j *LinkyJob) fetchDailyConsumption(ctx context.Context, start, end string) ([]consoReading, error) {
	url := fmt.Sprintf("%s/daily_consumption?prm=%s&start=%s&end=%s", consoAPIBase, j.prm, start, end)
	return j.doRequest(ctx, url, false)
}

func (j *LinkyJob) fetchHourlyConsumption(ctx context.Context, start, end string) ([]consoReading, error) {
	url := fmt.Sprintf("%s/consumption_load_curve?prm=%s&start=%s&end=%s", consoAPIBase, j.prm, start, end)
	return j.doRequest(ctx, url, true)
}

func (j *LinkyJob) doRequest(ctx context.Context, url string, hourly bool) ([]consoReading, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+j.token)

	resp, err := j.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("conso API returned %d", resp.StatusCode)
	}

	var raw consoResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	readings := make([]consoReading, 0, len(raw.IntervalReading))
	for _, item := range raw.IntervalReading {
		var wh float64
		fmt.Sscanf(item.Value, "%f", &wh)
		kwh := wh / 1000.0 // Conso API returns Wh

		reading := consoReading{Value: kwh}

		if hourly && len(item.DateDebut) >= 16 {
			// "2025-05-06T14:30:00" → date="2025-05-06", hour=14
			reading.Date = item.DateDebut[:10]
			var h int
			fmt.Sscanf(item.DateDebut[11:13], "%d", &h)
			reading.Hour = h
		} else {
			reading.Date = item.DateDebut[:10]
		}

		readings = append(readings, reading)
	}
	return readings, nil
}

// nextRunTime calculates when to next run — 08:34 local time.
func (j *LinkyJob) nextRunTime() time.Time {
	now := time.Now().In(j.location)
	target := time.Date(now.Year(), now.Month(), now.Day(), 8, 31, 0, 0, j.location)
	if !target.After(now) {
		target = target.AddDate(0, 0, 1)
	}
	return target
}
