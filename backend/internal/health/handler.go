package health

import (
	"encoding/json"
	"net/http"
	"time"
)

// Response is the JSON body returned by /health and /ready.
type Response struct {
	Status    string            `json:"status"` // "ok" | "degraded"
	Service   string            `json:"service"`
	Version   string            `json:"version"`
	Timestamp string            `json:"timestamp"`
	Checks    map[string]string `json:"checks,omitempty"` // future: db, cache…
}

// Handler holds the dependencies needed to evaluate health.
type Handler struct {
	serviceName    string
	serviceVersion string

	// checkers is a map of named readiness checks.
	// Each check returns "" on success or an error description on failure.
	// Add database ping, cache ping, etc. here as the project grows.
	checkers map[string]func() string
}

// New creates a Handler.
func New(serviceName, serviceVersion string) *Handler {
	return &Handler{
		serviceName:    serviceName,
		serviceVersion: serviceVersion,
		checkers:       make(map[string]func() string),
	}
}

// AddCheck registers a named readiness check.
// Usage: h.AddCheck("database", db.Ping)
func (h *Handler) AddCheck(name string, fn func() string) {
	h.checkers[name] = fn
}

// LivenessHandler handles GET /health.
// Returns 200 as long as the process is alive — no dependency checks.
// Used by Docker / k8s to decide whether to restart the container.
func (h *Handler) LivenessHandler(w http.ResponseWriter, r *http.Request) {
	h.write(w, http.StatusOK, Response{
		Status:    "ok",
		Service:   h.serviceName,
		Version:   h.serviceVersion,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}

// ReadinessHandler handles GET /ready.
// Runs all registered checks and returns 200 only when every check passes.
// Used by load balancers / k8s to decide whether to send traffic.
func (h *Handler) ReadinessHandler(w http.ResponseWriter, r *http.Request) {
	checks := make(map[string]string, len(h.checkers))
	status := "ok"
	httpStatus := http.StatusOK

	for name, fn := range h.checkers {
		if result := fn(); result != "" {
			checks[name] = result
			status = "degraded"
			httpStatus = http.StatusServiceUnavailable
		} else {
			checks[name] = "ok"
		}
	}

	h.write(w, httpStatus, Response{
		Status:    status,
		Service:   h.serviceName,
		Version:   h.serviceVersion,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Checks:    checks,
	})
}

func (h *Handler) write(w http.ResponseWriter, code int, body Response) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}
