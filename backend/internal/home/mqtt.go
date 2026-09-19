package home

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// MQTTSubscriber connects to the Mosquitto broker (running alongside tydom2mqtt)
// and converts incoming Tydom messages into service ingest calls.
//
// Topic structure published by tydom2mqtt:
//
//	tydom/{device_id}/temperature   → {"temperature": 20.5}
//	tydom/{device_id}/thermSensor   → {"thermSensor": {"celsius": 20.5}}
//	tydom/{device_id}/position      → {"position": 75}
//	tydom/exterior/temperature      → {"temperature": 12.3}
//	tydom/sunlight                  → {"lux": 3500}
//
// Topic names vary slightly by firmware version — adjust the handlers below
// to match what your broker actually publishes (use `mosquitto_sub -t '#'` to inspect).
type MQTTSubscriber struct {
	svc    *Service
	logger *slog.Logger
	broker string
	client mqtt.Client
}

func NewMQTTSubscriber(svc *Service, logger *slog.Logger, brokerURL string) (*MQTTSubscriber, error) {
	return &MQTTSubscriber{
		svc:    svc,
		logger: logger,
		broker: brokerURL,
	}, nil
}

// Start connects to the broker and subscribes to all Tydom topics.
// Runs in a goroutine and reconnects automatically on disconnect.
// Exits when ctx is cancelled.
func (m *MQTTSubscriber) Start(ctx context.Context) {
	go m.run(ctx)
}

func (m *MQTTSubscriber) run(ctx context.Context) {
	opts := mqtt.NewClientOptions().
		AddBroker(m.broker).
		SetClientID("personal-os-gateway").
		SetCleanSession(true).
		SetAutoReconnect(true).
		SetConnectRetryInterval(10 * time.Second).
		SetOnConnectHandler(m.onConnect).
		SetConnectionLostHandler(func(_ mqtt.Client, err error) {
			m.logger.Warn("MQTT connection lost", slog.Any("error", err))
		})

	m.client = mqtt.NewClient(opts)

	token := m.client.Connect()
	token.Wait()
	if err := token.Error(); err != nil {
		m.logger.Error("MQTT initial connect failed", slog.Any("error", err))
		// Keep the goroutine alive — AutoReconnect will retry.
	}

	<-ctx.Done()
	m.client.Disconnect(500) // wait up to 500ms for in-flight messages
	m.logger.Info("MQTT subscriber stopped")
}

func (m *MQTTSubscriber) onConnect(client mqtt.Client) {
	m.logger.Info("MQTT connected", slog.String("broker", m.broker))

	// Subscribe to all Tydom topics with QoS 1 (at-least-once delivery).
	// The '#' wildcard matches everything under 'tydom/'.
	client.Subscribe("tydom/#", 1, m.dispatch)
}

// dispatch routes incoming MQTT messages to the correct handler based on topic.
func (m *MQTTSubscriber) dispatch(_ mqtt.Client, msg mqtt.Message) {
	topic := msg.Topic()
	payload := msg.Payload()

	m.logger.Debug("MQTT message received",
		slog.String("topic", topic),
		slog.Int("bytes", len(payload)),
	)

	parts := strings.Split(topic, "/")
	if len(parts) < 2 {
		return
	}

	// tydom/sunlight
	if len(parts) == 2 && parts[1] == "sunlight" {
		m.handleSunlight(payload)
		return
	}

	// tydom/{room}/temperature or tydom/{room}/thermSensor
	if len(parts) == 3 {
		room := parts[1]
		switch parts[2] {
		case "temperature", "thermSensor":
			m.handleTemperature(room, payload)
		case "position":
			m.handleShutter(room, payload)
		}
	}
}

// ── Payload parsers ───────────────────────────────────────────────────────────
// These are lenient — if the payload doesn't match, we log and move on rather
// than crashing the subscriber.

func (m *MQTTSubscriber) handleTemperature(room string, payload []byte) {
	var msg struct {
		Temperature float64 `json:"temperature"`
		ThermSensor struct {
			Celsius float64 `json:"celsius"`
		} `json:"thermSensor"`
	}
	if err := json.Unmarshal(payload, &msg); err != nil {
		m.logger.Warn("malformed temperature payload",
			slog.String("room", room), slog.Any("error", err))
		return
	}

	celsius := msg.Temperature
	if celsius == 0 && msg.ThermSensor.Celsius != 0 {
		celsius = msg.ThermSensor.Celsius
	}
	if celsius == 0 {
		return
	}

	source := "interior"
	if room == "exterior" || room == "exterieur" {
		source = "exterior"
	}

	req := IngestTemperatureRequest{
		RoomName:   room,
		Celsius:    celsius,
		Source:     source,
		RecordedAt: time.Now().UTC().Format(time.RFC3339),
	}

	if _, err := m.svc.IngestTemperature(req); err != nil {
		m.logger.Error("ingest temperature failed",
			slog.String("room", room), slog.Any("error", err))
	}
}

func (m *MQTTSubscriber) handleShutter(room string, payload []byte) {
	var msg struct {
		Position int `json:"position"`
	}
	if err := json.Unmarshal(payload, &msg); err != nil {
		m.logger.Warn("malformed shutter payload", slog.Any("error", err))
		return
	}

	req := IngestShutterRequest{
		RoomName:   room,
		Position:   msg.Position,
		RecordedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if _, err := m.svc.IngestShutter(req); err != nil {
		m.logger.Error("ingest shutter failed", slog.Any("error", err))
	}
}

func (m *MQTTSubscriber) handleSunlight(payload []byte) {
	var msg struct {
		Lux float64 `json:"lux"`
	}
	if err := json.Unmarshal(payload, &msg); err != nil {
		m.logger.Warn("malformed sunlight payload", slog.Any("error", err))
		return
	}
	if _, err := m.svc.IngestSunlight(msg.Lux, time.Now().UTC().Format(time.RFC3339)); err != nil {
		m.logger.Error("ingest sunlight failed", slog.Any("error", err))
	}
}
