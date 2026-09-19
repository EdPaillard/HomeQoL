package home

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"homeqol/internal/db"
)

// Repository handles all SQL for the home monitor domain.
type Repository struct {
	db *db.DB
}

func NewRepository(database *db.DB) *Repository {
	return &Repository{db: database}
}

// ── Rooms ─────────────────────────────────────────────────────────────────────

func (r *Repository) ListRooms() ([]Room, error) {
	rows, err := r.db.Query(`SELECT id, name, external_id, created_at FROM tydom_rooms ORDER BY name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	rooms := []Room{}
	for rows.Next() {
		var rm Room
		var extID sql.NullString
		var createdAt string
		if err := rows.Scan(&rm.ID, &rm.Name, &extID, &createdAt); err != nil {
			return nil, err
		}
		if extID.Valid {
			rm.ExternalID = &extID.String
		}
		rm.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		rooms = append(rooms, rm)
	}
	return rooms, rows.Err()
}

func (r *Repository) FindRoomByName(name string) (string, error) {
	var id string
	err := r.db.QueryRow(`SELECT id FROM tydom_rooms WHERE name=?`, name).Scan(&id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", err
	}
	return id, nil
}

func (r *Repository) CreateRoom(id, name string) error {
	_, err := r.db.Exec(`INSERT INTO tydom_rooms (id, name) VALUES (?, ?)`, id, name)
	return err
}

// ── Temperature ───────────────────────────────────────────────────────────────

func (r *Repository) InsertTemperature(id, roomID string, celsius float64, source, recordedAt string) (*TemperatureReading, error) {
	_, err := r.db.Exec(
		`INSERT INTO tydom_temperature_readings (id, room_id, celsius, source, recorded_at) VALUES (?,?,?,?,?)`,
		id, roomID, celsius, source, recordedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert temperature: %w", err)
	}
	return &TemperatureReading{
		ID: id, RoomID: roomID, Celsius: celsius,
		Source: source, RecordedAt: parseTime(recordedAt),
	}, nil
}

// LatestTemperaturePerRoom returns the most recent reading per room,
// joining the room name for convenient display.
func (r *Repository) LatestTemperaturePerRoom() ([]TemperatureReading, error) {
	const q = `
		SELECT t.id, t.room_id, rm.name, t.celsius, t.source, t.recorded_at
		FROM tydom_temperature_readings t
		JOIN tydom_rooms rm ON rm.id = t.room_id
		WHERE t.id IN (
			SELECT id FROM tydom_temperature_readings t2
			WHERE t2.room_id = t.room_id
			ORDER BY t2.recorded_at DESC LIMIT 1
		)
		ORDER BY rm.name ASC`
	rows, err := r.db.Query(q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return r.scanTemperatures(rows)
}

func (r *Repository) TemperatureHistory(roomID, from, to string) ([]TemperatureReading, error) {
	const q = `
		SELECT t.id, t.room_id, rm.name, t.celsius, t.source, t.recorded_at
		FROM tydom_temperature_readings t
		JOIN tydom_rooms rm ON rm.id = t.room_id
		WHERE t.room_id=? AND t.recorded_at >= ? AND t.recorded_at <= ?
		ORDER BY t.recorded_at ASC`
	rows, err := r.db.Query(q, roomID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return r.scanTemperatures(rows)
}

func (r *Repository) scanTemperatures(rows *sql.Rows) ([]TemperatureReading, error) {
	var readings []TemperatureReading
	for rows.Next() {
		var t TemperatureReading
		var recordedAt string
		if err := rows.Scan(&t.ID, &t.RoomID, &t.RoomName, &t.Celsius, &t.Source, &recordedAt); err != nil {
			return nil, err
		}
		t.RecordedAt = parseTime(recordedAt)
		readings = append(readings, t)
	}
	return readings, rows.Err()
}

// ── Shutters ──────────────────────────────────────────────────────────────────

func (r *Repository) InsertShutter(id, roomID string, position int, recordedAt string) (*ShutterReading, error) {
	_, err := r.db.Exec(
		`INSERT INTO tydom_shutter_readings (id, room_id, position, recorded_at) VALUES (?,?,?,?)`,
		id, roomID, position, recordedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert shutter: %w", err)
	}
	return &ShutterReading{
		ID: id, RoomID: roomID, Position: position, RecordedAt: parseTime(recordedAt),
	}, nil
}

func (r *Repository) LatestShutterPerRoom() ([]ShutterReading, error) {
	const q = `
		SELECT s.id, s.room_id, rm.name, s.position, s.recorded_at
		FROM tydom_shutter_readings s
		JOIN tydom_rooms rm ON rm.id = s.room_id
		WHERE s.id IN (
			SELECT id FROM tydom_shutter_readings s2
			WHERE s2.room_id = s.room_id
			ORDER BY s2.recorded_at DESC LIMIT 1
		)
		ORDER BY rm.name ASC`
	rows, err := r.db.Query(q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var readings []ShutterReading
	for rows.Next() {
		var s ShutterReading
		var recordedAt string
		if err := rows.Scan(&s.ID, &s.RoomID, &s.RoomName, &s.Position, &recordedAt); err != nil {
			return nil, err
		}
		s.RecordedAt = parseTime(recordedAt)
		readings = append(readings, s)
	}
	return readings, rows.Err()
}

// ── Sunlight ──────────────────────────────────────────────────────────────────

func (r *Repository) InsertSunlight(id string, lux float64, recordedAt string) (*SunlightReading, error) {
	_, err := r.db.Exec(
		`INSERT INTO tydom_sunlight_readings (id, lux, recorded_at) VALUES (?,?,?)`,
		id, lux, recordedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert sunlight: %w", err)
	}
	return &SunlightReading{ID: id, Lux: lux, RecordedAt: parseTime(recordedAt)}, nil
}

func (r *Repository) LatestSunlight() (*SunlightReading, error) {
	var s SunlightReading
	var recordedAt string
	err := r.db.QueryRow(
		`SELECT id, lux, recorded_at FROM tydom_sunlight_readings ORDER BY recorded_at DESC LIMIT 1`,
	).Scan(&s.ID, &s.Lux, &recordedAt)
	if err != nil {
		return nil, err
	}
	s.RecordedAt = parseTime(recordedAt)
	return &s, nil
}

// ── Energy ────────────────────────────────────────────────────────────────────

func (r *Repository) InsertEnergy(id string, req IngestEnergyRequest) (*EnergyReading, error) {
	_, err := r.db.Exec(
		`INSERT INTO linky_energy_readings (id, reading_date, reading_hour, kwh, granularity)
		 VALUES (?,?,?,?,?)`,
		id, req.Date, req.Hour, req.KWh, req.Granularity,
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return nil, ErrDuplicateReading
		}
		return nil, fmt.Errorf("insert energy: %w", err)
	}
	return &EnergyReading{
		ID: id, ReadingDate: req.Date, ReadingHour: req.Hour,
		KWh: req.KWh, Granularity: req.Granularity, CreatedAt: time.Now(),
	}, nil
}

func (r *Repository) EnergyForDate(date string) (float64, error) {
	var kwh float64
	err := r.db.QueryRow(
		`SELECT COALESCE(SUM(kwh),0) FROM linky_energy_readings WHERE reading_date=? AND granularity='daily'`,
		date,
	).Scan(&kwh)
	return kwh, err
}

func (r *Repository) EnergyBetween(from, to string) (float64, error) {
	var kwh float64
	err := r.db.QueryRow(
		`SELECT COALESCE(SUM(kwh),0) FROM linky_energy_readings WHERE reading_date>=? AND reading_date<=? AND granularity='daily'`,
		from, to,
	).Scan(&kwh)
	return kwh, err
}

func (r *Repository) EnergyHistory(from, to, granularity string) ([]EnergyReading, error) {
	const q = `
		SELECT id, reading_date, reading_hour, kwh, granularity, created_at
		FROM linky_energy_readings
		WHERE reading_date>=? AND reading_date<=? AND granularity=?
		ORDER BY reading_date ASC, reading_hour ASC`
	rows, err := r.db.Query(q, from, to, granularity)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var readings []EnergyReading
	for rows.Next() {
		var e EnergyReading
		var hour sql.NullInt64
		var createdAt string
		if err := rows.Scan(&e.ID, &e.ReadingDate, &hour, &e.KWh, &e.Granularity, &createdAt); err != nil {
			return nil, err
		}
		if hour.Valid {
			h := int(hour.Int64)
			e.ReadingHour = &h
		}
		e.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		readings = append(readings, e)
	}
	return readings, rows.Err()
}

// ── Alerts ────────────────────────────────────────────────────────────────────

func (r *Repository) CreateAlert(id, alertType, severity, title, body string, sourceTable, sourceID *string) error {
	_, err := r.db.Exec(
		`INSERT INTO alerts (id, type, severity, title, body, source_table, source_id) VALUES (?,?,?,?,?,?,?)`,
		id, alertType, severity, title, body, sourceTable, sourceID,
	)
	return err
}

func (r *Repository) ListAlerts(unreadOnly bool) ([]Alert, error) {
	q := `SELECT id, type, severity, title, body, source_table, source_id, acknowledged_at, created_at
		  FROM alerts`
	if unreadOnly {
		q += ` WHERE acknowledged_at IS NULL`
	}
	q += ` ORDER BY created_at DESC LIMIT 100`
	rows, err := r.db.Query(q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var alerts []Alert
	for rows.Next() {
		var a Alert
		var sourceTable, sourceID, acknowledgedAt sql.NullString
		var createdAt string
		if err := rows.Scan(
			&a.ID, &a.Type, &a.Severity, &a.Title, &a.Body,
			&sourceTable, &sourceID, &acknowledgedAt, &createdAt,
		); err != nil {
			return nil, err
		}
		if sourceTable.Valid {
			a.SourceTable = &sourceTable.String
		}
		if sourceID.Valid {
			a.SourceID = &sourceID.String
		}
		if acknowledgedAt.Valid {
			t, _ := time.Parse(time.RFC3339, acknowledgedAt.String)
			a.AcknowledgedAt = &t
		}
		a.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		alerts = append(alerts, a)
	}
	return alerts, rows.Err()
}

func (r *Repository) UnreadAlertCount() (int, error) {
	var n int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM alerts WHERE acknowledged_at IS NULL`).Scan(&n)
	return n, err
}

func (r *Repository) AcknowledgeAlert(id string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := r.db.Exec(
		`UPDATE alerts SET acknowledged_at=? WHERE id=? AND acknowledged_at IS NULL`, now, id,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// ── Helper ────────────────────────────────────────────────────────────────────

func parseTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}
