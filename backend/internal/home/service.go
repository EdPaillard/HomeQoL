package home

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

// Service handles the home monitor domain.
//
// It has two ingest paths:
//  1. MQTT (Tydom): the MQTTSubscriber calls IngestTemperature/Shutter/Sunlight
//     in real-time as messages arrive from tydom2mqtt.
//  2. HTTP cron (Linky): a daily scheduler calls IngestEnergy at 08:34.
//
// After each ingest, threshold checks fire synchronously and may create alerts.
type Service struct {
	repo       *Repository
	logger     *slog.Logger
	thresholds AlertThresholds
	now        func() time.Time
}

func NewService(repo *Repository, logger *slog.Logger, thresholds AlertThresholds) *Service {
	return &Service{
		repo:       repo,
		logger:     logger,
		thresholds: thresholds,
		now:        time.Now,
	}
}

// ── Dashboard ─────────────────────────────────────────────────────────────────

func (s *Service) Dashboard() (*DashboardSnapshot, error) {
	snap := &DashboardSnapshot{GeneratedAt: s.now()}

	rooms, err := s.repo.ListRooms()
	if err != nil {
		return nil, err
	}
	snap.Rooms = rooms

	temps, err := s.repo.LatestTemperaturePerRoom()
	if err != nil {
		return nil, err
	}
	snap.LatestTemps = temps

	shutters, err := s.repo.LatestShutterPerRoom()
	if err != nil {
		return nil, err
	}
	snap.LatestShutters = shutters

	sun, err := s.repo.LatestSunlight()
	if err == nil {
		snap.LatestSunlight = sun
	}

	today := s.now().Format("2006-01-02")
	if kwh, err := s.repo.EnergyForDate(today); err == nil {
		snap.TodayKWh = &kwh
	}

	now := s.now()
	firstOfMonth := fmt.Sprintf("%d-%02d-01", now.Year(), now.Month())
	if kwh, err := s.repo.EnergyBetween(firstOfMonth, today); err == nil {
		snap.MonthKWh = &kwh
	}

	unread, _ := s.repo.UnreadAlertCount()
	snap.UnreadAlerts = unread

	return snap, nil
}

// ── Ingest — Tydom ────────────────────────────────────────────────────────────

// IngestTemperature stores a temperature reading and checks thresholds.
// Called by the MQTT subscriber whenever Tydom publishes a temperature message.
func (s *Service) IngestTemperature(req IngestTemperatureRequest) (*TemperatureReading, error) {
	roomID, err := s.getOrCreateRoom(req.RoomName)
	if err != nil {
		return nil, err
	}

	id := uuid.New().String()
	reading, err := s.repo.InsertTemperature(id, roomID, req.Celsius, req.Source, req.RecordedAt)
	if err != nil {
		return nil, err
	}

	// Threshold checks — run synchronously, don't block ingest on failure.
	if err := s.checkTemperatureAlert(req.RoomName, id, req.Celsius); err != nil {
		s.logger.Warn("alert check failed", slog.Any("error", err))
	}

	return reading, nil
}

// IngestShutter stores a shutter position reading.
func (s *Service) IngestShutter(req IngestShutterRequest) (*ShutterReading, error) {
	roomID, err := s.getOrCreateRoom(req.RoomName)
	if err != nil {
		return nil, err
	}
	id := uuid.New().String()
	return s.repo.InsertShutter(id, roomID, req.Position, req.RecordedAt)
}

// IngestSunlight stores a sunlight reading.
func (s *Service) IngestSunlight(lux float64, recordedAt string) (*SunlightReading, error) {
	id := uuid.New().String()
	return s.repo.InsertSunlight(id, lux, recordedAt)
}

// ── Ingest — Linky ────────────────────────────────────────────────────────────

// IngestEnergy stores a Linky energy reading.
// The UNIQUE constraint in SQLite prevents duplicate imports — if the date
// already exists, this is a no-op (returns ErrDuplicateReading).
func (s *Service) IngestEnergy(req IngestEnergyRequest) (*EnergyReading, error) {
	id := uuid.New().String()
	reading, err := s.repo.InsertEnergy(id, req)
	if err != nil {
		return nil, err
	}

	// Check for energy spike alert.
	if req.Granularity == "daily" {
		if err := s.checkEnergySpikeAlert(id, req.Date, req.KWh); err != nil {
			s.logger.Warn("energy alert check failed", slog.Any("error", err))
		}
	}

	return reading, nil
}

// ── History queries ───────────────────────────────────────────────────────────

func (s *Service) TemperatureHistory(roomID, from, to string) ([]TemperatureReading, error) {
	return s.repo.TemperatureHistory(roomID, from, to)
}

func (s *Service) EnergyHistory(from, to, granularity string) ([]EnergyReading, error) {
	return s.repo.EnergyHistory(from, to, granularity)
}

// ── Alerts ────────────────────────────────────────────────────────────────────

func (s *Service) ListAlerts(unreadOnly bool) ([]Alert, error) {
	return s.repo.ListAlerts(unreadOnly)
}

func (s *Service) AcknowledgeAlert(id string) error {
	return s.repo.AcknowledgeAlert(id)
}

// ── MQTT subscriber wiring ────────────────────────────────────────────────────

// StartMQTTSubscriber launches the goroutine that reads from the Tydom MQTT
// broker and calls the ingest methods above.
// See mqtt.go for the implementation.
func (s *Service) StartMQTTSubscriber(ctx context.Context, brokerURL string) error {
	sub, err := NewMQTTSubscriber(s, s.logger, brokerURL)
	if err != nil {
		return fmt.Errorf("create MQTT subscriber: %w", err)
	}
	sub.Start(ctx)
	return nil
}

// ── Linky cron ────────────────────────────────────────────────────────────────

// StartLinkyJob launches the daily goroutine that fetches Linky data from Conso API.
func (s *Service) StartLinkyJob(ctx context.Context, consoAPIToken, prm string, loc *time.Location) {
	job := NewLinkyJob(s, s.logger, consoAPIToken, prm, loc)
	job.Start(ctx)
}

// ── Internal helpers ──────────────────────────────────────────────────────────

// getOrCreateRoom finds a room by name or creates it.
func (s *Service) getOrCreateRoom(name string) (string, error) {
	id, err := s.repo.FindRoomByName(name)
	if err == nil {
		return id, nil
	}
	// Create new room.
	newID := uuid.New().String()
	if err := s.repo.CreateRoom(newID, name); err != nil {
		return "", fmt.Errorf("create room %q: %w", name, err)
	}
	s.logger.Info("new room created", slog.String("name", name), slog.String("id", newID))
	return newID, nil
}

func (s *Service) checkTemperatureAlert(roomName, sourceID string, celsius float64) error {
	var alertType, title, body string
	var severity string

	switch {
	case celsius < s.thresholds.TempLowCelsius:
		alertType = "temperature_low"
		severity = "warning"
		title = fmt.Sprintf("Température basse — %s", roomName)
		body = fmt.Sprintf("%.1f°C détecté dans %s (seuil : %.0f°C)", celsius, roomName, s.thresholds.TempLowCelsius)
	case celsius > s.thresholds.TempHighCelsius:
		alertType = "temperature_high"
		severity = "warning"
		title = fmt.Sprintf("Température élevée — %s", roomName)
		body = fmt.Sprintf("%.1f°C détecté dans %s (seuil : %.0f°C)", celsius, roomName, s.thresholds.TempHighCelsius)
	default:
		return nil // within range, no alert
	}

	sourceTable := "tydom_temperature_readings"
	id := uuid.New().String()
	return s.repo.CreateAlert(id, alertType, severity, title, body, &sourceTable, &sourceID)
}

func (s *Service) checkEnergySpikeAlert(sourceID, date string, kwh float64) error {
	if kwh <= s.thresholds.EnergySpikeKWh {
		return nil
	}
	sourceTable := "linky_energy_readings"
	id := uuid.New().String()
	return s.repo.CreateAlert(
		id,
		"energy_spike",
		"info",
		"Consommation élevée",
		fmt.Sprintf("%.1f kWh consommés le %s (seuil : %.0f kWh)", kwh, date, s.thresholds.EnergySpikeKWh),
		&sourceTable,
		&sourceID,
	)
}
