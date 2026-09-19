package config

import (
	"os"
	"path/filepath"
	"time"
)

// Config holds all runtime configuration loaded from environment variables.
type Config struct {
	// HTTP
	Port string

	// Environment: "development" | "production"
	Env string

	// Service metadata (surfaced on /health)
	ServiceName    string
	ServiceVersion string

	// Database
	DBPath string

	// Timezone for streak and cron calculations (e.g. "Europe/Paris")
	TZ string

	// Home monitor
	MQTTBrokerURL string // e.g. "tcp://localhost:1883"

	// Linky / Conso API
	ConsoAPIToken string // Bearer token from https://conso.boris.sh
	PRM           string
}

// Load reads configuration from environment variables, falling back to defaults.
func Load() Config {
	dbPath := getEnv("DB_PATH", "./data/personal-os.db")
	if abs, err := filepath.Abs(dbPath); err == nil {
		dbPath = abs
	}
	return Config{
		Port:           getEnv("PORT", "8080"),
		Env:            getEnv("ENV", "development"),
		ServiceName:    getEnv("SERVICE_NAME", "personal-os-gateway"),
		ServiceVersion: getEnv("SERVICE_VERSION", "0.1.0"),
		DBPath:         dbPath,
		TZ:             getEnv("TZ", "Europe/Paris"),
		MQTTBrokerURL:  getEnv("MQTT_BROKER_URL", "tcp://localhost:1883"),
		ConsoAPIToken:  getEnv("CONSO_API_TOKEN", ""),
		PRM:            getEnv("PRM", ""),
	}
}

// IsDevelopment returns true when running in local dev mode.
func (c Config) IsDevelopment() bool { return c.Env == "development" }

// Addr returns the TCP address string for http.ListenAndServe.
func (c Config) Addr() string { return ":" + c.Port }

// Location parses the TZ field into a *time.Location.
// Falls back to time.Local if the value is empty or invalid.
func (c Config) Location() *time.Location {
	if c.TZ == "" {
		return time.Local
	}
	loc, err := time.LoadLocation(c.TZ)
	if err != nil {
		return time.Local
	}
	return loc
}

// MQTTEnabled returns true when a broker URL is configured.
func (c Config) MQTTEnabled() bool { return c.MQTTBrokerURL != "" }

// LinkyEnabled returns true when a Conso API token is configured.
func (c Config) LinkyEnabled() bool { return c.ConsoAPIToken != "" }

// ── helpers ───────────────────────────────────────────────────────────────────

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
