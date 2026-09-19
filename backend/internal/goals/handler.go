package goals

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
)

type Handler struct{ svc *Service }

// NewRouter mounts all goal routes onto a Chi sub-router.
func NewRouter(svc *Service) http.Handler {
	h := &Handler{svc: svc}
	r := chi.NewRouter()
	r.Get("/", h.List)
	r.Post("/", h.Create)
	r.Route("/{id}", func(r chi.Router) {
		r.Get("/", h.Get)
		r.Patch("/", h.Update)
		r.Delete("/", h.Delete)
	})
	return r
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	goals, err := h.svc.List()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list goals", err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"goals": goals, "count": len(goals)})
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	g, err := h.svc.Get(chi.URLParam(r, "id"))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respondError(w, http.StatusNotFound, "goal not found", nil)
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to get goal", err)
		return
	}
	respondJSON(w, http.StatusOK, g)
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}
	g, err := h.svc.Create(req)
	if err != nil {
		if isValidation(err) {
			respondError(w, http.StatusBadRequest, err.Error(), nil)
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to create goal", err)
		return
	}
	respondJSON(w, http.StatusCreated, g)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	var req UpdateRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}
	g, err := h.svc.Update(chi.URLParam(r, "id"), req)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respondError(w, http.StatusNotFound, "goal not found", nil)
			return
		}
		if isValidation(err) {
			respondError(w, http.StatusBadRequest, err.Error(), nil)
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to update goal", err)
		return
	}
	respondJSON(w, http.StatusOK, g)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Delete(chi.URLParam(r, "id")); err != nil {
		if errors.Is(err, ErrNotFound) {
			respondError(w, http.StatusNotFound, "goal not found", nil)
			return
		}
		if errors.Is(err, ErrHasChildren) {
			respondError(w, http.StatusConflict, "delete milestones before deleting this goal", nil)
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to delete goal", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── shared helpers ────────────────────────────────────────────────────────────

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
	return !errors.Is(err, ErrNotFound) && !errors.Is(err, ErrHasChildren)
}
