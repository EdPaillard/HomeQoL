// Package home implements the home monitor domain:
// temperature/shutter/sunlight readings from Tydom via MQTT,
// energy readings from Linky via Conso API, and alerts.
package home

import (
	"errors"
	"time"
)

// ── Room ──────────────────────────────────────────────────────────────────────

type Room struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	ExternalID *string   `json:"external_id,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

// ── Readings ──────────────────────────────────────────────────────────────────

type TemperatureReading struct {
	ID         string    `json:"id"`
	RoomID     string    `json:"room_id"`
	RoomName   string    `json:"room_name,omitempty"` // joined for API responses
	Celsius    float64   `json:"celsius"`
	Source     string    `json:"source"` // "interior" | "exterior"
	RecordedAt time.Time `json:"recorded_at"`
}

type ShutterReading struct {
	ID         string    `json:"id"`
	RoomID     string    `json:"room_id"`
	RoomName   string    `json:"room_name,omitempty"`
	Position   int       `json:"position"` // 0=closed, 100=open
	RecordedAt time.Time `json:"recorded_at"`
}

type SunlightReading struct {
	ID         string    `json:"id"`
	Lux        float64   `json:"lux"`
	RecordedAt time.Time `json:"recorded_at"`
}

type EnergyReading struct {
	ID          string    `json:"id"`
	ReadingDate string    `json:"reading_date"`           // "YYYY-MM-DD"
	ReadingHour *int      `json:"reading_hour,omitempty"` // 0-23, nil for daily
	KWh         float64   `json:"kwh"`
	Granularity string    `json:"granularity"` // "daily" | "hourly"
	CreatedAt   time.Time `json:"created_at"`
}

// ── Alerts ────────────────────────────────────────────────────────────────────

type Alert struct {
	ID             string     `json:"id"`
	Type           string     `json:"type"`
	Severity       string     `json:"severity"`
	Title          string     `json:"title"`
	Body           string     `json:"body"`
	SourceTable    *string    `json:"source_table,omitempty"`
	SourceID       *string    `json:"source_id,omitempty"`
	AcknowledgedAt *time.Time `json:"acknowledged_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

func (a *Alert) IsRead() bool { return a.AcknowledgedAt != nil }

// ── Dashboard snapshot ────────────────────────────────────────────────────────

// DashboardSnapshot is the single response for GET /api/v1/home/dashboard.
// Combines all latest readings into one payload so the React/Flutter client
// makes one call instead of four.
type DashboardSnapshot struct {
	Rooms          []Room               `json:"rooms"`
	LatestTemps    []TemperatureReading `json:"latest_temperatures"`
	LatestShutters []ShutterReading     `json:"latest_shutters"`
	LatestSunlight *SunlightReading     `json:"latest_sunlight,omitempty"`
	TodayKWh       *float64             `json:"today_kwh,omitempty"`
	MonthKWh       *float64             `json:"month_kwh,omitempty"`
	UnreadAlerts   int                  `json:"unread_alerts"`
	GeneratedAt    time.Time            `json:"generated_at"`
}

// ── Request DTOs ──────────────────────────────────────────────────────────────

type IngestTemperatureRequest struct {
	RoomName   string  `json:"room_name"`
	Celsius    float64 `json:"celsius"`
	Source     string  `json:"source"`      // "interior" | "exterior"
	RecordedAt string  `json:"recorded_at"` // ISO-8601
}

type IngestShutterRequest struct {
	RoomName   string `json:"room_name"`
	Position   int    `json:"position"`
	RecordedAt string `json:"recorded_at"`
}

type IngestEnergyRequest struct {
	Date        string  `json:"date"` // "YYYY-MM-DD"
	Hour        *int    `json:"hour"` // nil = daily total
	KWh         float64 `json:"kwh"`
	Granularity string  `json:"granularity"` // "daily" | "hourly"
}

type AlertThresholds struct {
	TempLowCelsius  float64 // below this in any room → alert
	TempHighCelsius float64 // above this → alert
	EnergySpikeKWh  float64 // daily kWh above this → alert
}

var DefaultThresholds = AlertThresholds{
	TempLowCelsius:  14.0,
	TempHighCelsius: 28.0,
	EnergySpikeKWh:  20.0,
}

// ── Sentinel errors ───────────────────────────────────────────────────────────

var (
	ErrNotFound         = errors.New("not found")
	ErrDuplicateReading = errors.New("reading already exists for this period")
)
