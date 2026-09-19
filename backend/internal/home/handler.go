package home

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
)

type Handler struct{ svc *Service }

// NewRouter mounts all home monitor routes.
func NewRouter(svc *Service) http.Handler {
	h := &Handler{svc: svc}
	r := chi.NewRouter()

	r.Get("/dashboard", h.Dashboard)

	r.Get("/temperature", h.TemperatureHistory)
	r.Get("/energy", h.EnergyHistory)

	r.Get("/alerts", h.ListAlerts)
	r.Post("/alerts/{id}/acknowledge", h.AcknowledgeAlert)

	// Internal ingest endpoints — called by MQTT subscriber or admin scripts.
	// In production you'd protect these with a shared secret middleware.
	r.Post("/ingest/temperature", h.IngestTemperature)
	r.Post("/ingest/shutter", h.IngestShutter)
	r.Post("/ingest/energy", h.IngestEnergy)

	return r
}

// GET /api/v1/home/dashboard
func (h *Handler) Dashboard(w http.ResponseWriter, r *http.Request) {
	snap, err := h.svc.Dashboard()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to build dashboard", err)
		return
	}
	respondJSON(w, http.StatusOK, snap)
}

// GET /api/v1/home/temperature?room_id=&from=&to=
func (h *Handler) TemperatureHistory(w http.ResponseWriter, r *http.Request) {
	roomID := r.URL.Query().Get("room_id")
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")
	if roomID == "" || from == "" || to == "" {
		respondError(w, http.StatusBadRequest, "room_id, from, and to are required", nil)
		return
	}
	readings, err := h.svc.TemperatureHistory(roomID, from, to)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get temperature history", err)
		return
	}
	if readings == nil {
		readings = []TemperatureReading{}
	}
	respondJSON(w, http.StatusOK, map[string]any{"readings": readings, "count": len(readings)})
}

// GET /api/v1/home/energy?from=&to=&granularity=daily
func (h *Handler) EnergyHistory(w http.ResponseWriter, r *http.Request) {
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")
	granularity := r.URL.Query().Get("granularity")
	if granularity == "" {
		granularity = "daily"
	}
	if from == "" || to == "" {
		respondError(w, http.StatusBadRequest, "from and to are required", nil)
		return
	}
	readings, err := h.svc.EnergyHistory(from, to, granularity)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get energy history", err)
		return
	}
	if readings == nil {
		readings = []EnergyReading{}
	}
	respondJSON(w, http.StatusOK, map[string]any{"readings": readings, "count": len(readings)})
}

// GET /api/v1/home/alerts?unread_only=true
func (h *Handler) ListAlerts(w http.ResponseWriter, r *http.Request) {
	unreadOnly := r.URL.Query().Get("unread_only") == "true"
	alerts, err := h.svc.ListAlerts(unreadOnly)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list alerts", err)
		return
	}
	if alerts == nil {
		alerts = []Alert{}
	}
	respondJSON(w, http.StatusOK, map[string]any{"alerts": alerts, "count": len(alerts)})
}

// POST /api/v1/home/alerts/{id}/acknowledge
func (h *Handler) AcknowledgeAlert(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.AcknowledgeAlert(chi.URLParam(r, "id")); err != nil {
		if errors.Is(err, ErrNotFound) {
			respondError(w, http.StatusNotFound, "alert not found", nil)
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to acknowledge alert", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// POST /api/v1/home/ingest/temperature
func (h *Handler) IngestTemperature(w http.ResponseWriter, r *http.Request) {
	var req IngestTemperatureRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid body", err)
		return
	}
	reading, err := h.svc.IngestTemperature(req)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "ingest failed", err)
		return
	}
	respondJSON(w, http.StatusCreated, reading)
}

// POST /api/v1/home/ingest/shutter
func (h *Handler) IngestShutter(w http.ResponseWriter, r *http.Request) {
	var req IngestShutterRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid body", err)
		return
	}
	reading, err := h.svc.IngestShutter(req)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "ingest failed", err)
		return
	}
	respondJSON(w, http.StatusCreated, reading)
}

// POST /api/v1/home/ingest/energy
func (h *Handler) IngestEnergy(w http.ResponseWriter, r *http.Request) {
	var req IngestEnergyRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid body", err)
		return
	}
	reading, err := h.svc.IngestEnergy(req)
	if err != nil {
		if errors.Is(err, ErrDuplicateReading) {
			respondError(w, http.StatusConflict, "reading already exists for this period", nil)
			return
		}
		respondError(w, http.StatusInternalServerError, "ingest failed", err)
		return
	}
	respondJSON(w, http.StatusCreated, reading)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

type errResp struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

func respondJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func respondError(w http.ResponseWriter, status int, message string, _ error) {
	respondJSON(w, status, errResp{Error: http.StatusText(status), Message: message})
}

func decodeJSON(r *http.Request, dst any) error {
	return json.NewDecoder(r.Body).Decode(dst)
}
