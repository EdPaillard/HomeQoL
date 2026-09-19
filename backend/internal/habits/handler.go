package habits

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// Handler is the HTTP layer for the habits domain.
type Handler struct {
	svc *Service
}

// NewRouter returns a mounted Chi sub-router for all habit endpoints.
func NewRouter(svc *Service) http.Handler {
	h := &Handler{svc: svc}
	r := chi.NewRouter()

	r.Get("/", h.List)    // GET  /api/v1/habits
	r.Post("/", h.Create) // POST /api/v1/habits

	r.Route("/{id}", func(r chi.Router) {
		r.Get("/", h.Get)       // GET    /api/v1/habits/{id}
		r.Patch("/", h.Update)  // PATCH  /api/v1/habits/{id}
		r.Delete("/", h.Delete) // DELETE /api/v1/habits/{id}

		// Completions are nested under the habit they belong to.
		r.Post("/complete", h.Complete)                    // POST   /api/v1/habits/{id}/complete
		r.Delete("/complete/{completionID}", h.Uncomplete) // DELETE /api/v1/habits/{id}/complete/{completionID}
	})

	return r
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	habits, err := h.svc.List()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list habits", err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"habits": habits, "count": len(habits)})
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	habit, err := h.svc.Get(chi.URLParam(r, "id"))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respondError(w, http.StatusNotFound, "habit not found", nil)
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to get habit", err)
		return
	}
	respondJSON(w, http.StatusOK, habit)
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}
	habit, err := h.svc.Create(req)
	if err != nil {
		if isValidation(err) {
			respondError(w, http.StatusBadRequest, err.Error(), nil)
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to create habit", err)
		return
	}
	respondJSON(w, http.StatusCreated, habit)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	var req UpdateRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}
	habit, err := h.svc.Update(chi.URLParam(r, "id"), req)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respondError(w, http.StatusNotFound, "habit not found", nil)
			return
		}
		if isValidation(err) {
			respondError(w, http.StatusBadRequest, err.Error(), nil)
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to update habit", err)
		return
	}
	respondJSON(w, http.StatusOK, habit)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Delete(chi.URLParam(r, "id")); err != nil {
		if errors.Is(err, ErrNotFound) {
			respondError(w, http.StatusNotFound, "habit not found", nil)
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to delete habit", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Complete(w http.ResponseWriter, r *http.Request) {
	var req CompleteRequest
	// Body is optional — bare POST means "complete today with no note".
	_ = decodeJSON(r, &req)

	completion, err := h.svc.Complete(chi.URLParam(r, "id"), req)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respondError(w, http.StatusNotFound, "habit not found", nil)
			return
		}
		if errors.Is(err, ErrAlreadyDone) {
			respondError(w, http.StatusConflict, "habit already completed for this date", nil)
			return
		}
		if isValidation(err) {
			respondError(w, http.StatusBadRequest, err.Error(), nil)
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to log completion", err)
		return
	}
	respondJSON(w, http.StatusCreated, completion)
}

func (h *Handler) Uncomplete(w http.ResponseWriter, r *http.Request) {
	err := h.svc.Uncomplete(chi.URLParam(r, "id"), chi.URLParam(r, "completionID"))
	if err != nil {
		if errors.Is(err, ErrCompletionNotFound) {
			respondError(w, http.StatusNotFound, "completion not found", nil)
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to remove completion", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── shared helpers (same pattern as tasks/handler.go) ─────────────────────────

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

func isValidation(err error) bool {
	return !errors.Is(err, ErrNotFound) &&
		!errors.Is(err, ErrAlreadyDone) &&
		!errors.Is(err, ErrCompletionNotFound)
}
