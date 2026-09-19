package tasks

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

// Handler wires HTTP requests to the service.
// It is the only layer that knows about http.Request and http.ResponseWriter.
//
// Responsibilities:
//   - Decode JSON request bodies
//   - Parse and validate query parameters
//   - Call the service
//   - Map domain errors to HTTP status codes
//   - Encode JSON responses
//
// It does NOT contain business logic — that lives in the service.
type Handler struct {
	svc *Service
}

// NewHandler creates a Handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// NewRouter returns a Chi sub-router with all task routes mounted.
// Called from the main router: r.Mount("/api/v1/tasks", tasks.NewRouter(...))
func NewRouter(svc *Service) http.Handler {
	h := NewHandler(svc)
	r := chi.NewRouter()

	r.Get("/", h.List)            // GET  /api/v1/tasks
	r.Post("/", h.Create)         // POST /api/v1/tasks

	r.Route("/{id}", func(r chi.Router) {
		r.Get("/", h.Get)          // GET    /api/v1/tasks/{id}
		r.Patch("/", h.Update)     // PATCH  /api/v1/tasks/{id}
		r.Delete("/", h.Delete)    // DELETE /api/v1/tasks/{id}
		r.Post("/complete", h.Complete) // POST /api/v1/tasks/{id}/complete
	})

	return r
}

// ── Handlers ──────────────────────────────────────────────────────────────────

// List godoc
// GET /api/v1/tasks
// Query params: status, priority, goal_id, tag_id, q, due_today, overdue, limit, offset
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	f := ListFilter{}

	if s := r.URL.Query().Get("status"); s != "" {
		status := Status(s)
		f.Status = &status
	}
	if p := r.URL.Query().Get("priority"); p != "" {
		if n, err := strconv.Atoi(p); err == nil {
			pri := Priority(n)
			f.Priority = &pri
		}
	}
	if g := r.URL.Query().Get("goal_id"); g != "" {
		f.GoalID = &g
	}
	if t := r.URL.Query().Get("tag_id"); t != "" {
		f.TagID = &t
	}
	if q := r.URL.Query().Get("q"); q != "" {
		f.Search = q
	}
	f.DueToday = r.URL.Query().Get("due_today") == "true"
	f.Overdue = r.URL.Query().Get("overdue") == "true"

	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil {
			f.Limit = n
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if n, err := strconv.Atoi(o); err == nil {
			f.Offset = n
		}
	}

	tasks, err := h.svc.List(f)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list tasks", err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"tasks": tasks,
		"count": len(tasks),
	})
}

// Get godoc
// GET /api/v1/tasks/{id}
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	task, err := h.svc.Get(id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respondError(w, http.StatusNotFound, "task not found", nil)
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to get task", err)
		return
	}
	respondJSON(w, http.StatusOK, task)
}

// Create godoc
// POST /api/v1/tasks
// Body: CreateRequest JSON
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}

	task, err := h.svc.Create(req)
	if err != nil {
		if isValidationError(err) {
			respondError(w, http.StatusBadRequest, err.Error(), nil)
			return
		}
		if errors.Is(err, ErrTagNotFound) {
			respondError(w, http.StatusBadRequest, "one or more tag IDs do not exist", nil)
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to create task", err)
		return
	}

	respondJSON(w, http.StatusCreated, task)
}

// Update godoc
// PATCH /api/v1/tasks/{id}
// Body: UpdateRequest JSON (all fields optional)
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req UpdateRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}

	task, err := h.svc.Update(id, req)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respondError(w, http.StatusNotFound, "task not found", nil)
			return
		}
		if errors.Is(err, ErrTagNotFound) {
			respondError(w, http.StatusBadRequest, "one or more tag IDs do not exist", nil)
			return
		}
		if isValidationError(err) {
			respondError(w, http.StatusBadRequest, err.Error(), nil)
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to update task", err)
		return
	}

	respondJSON(w, http.StatusOK, task)
}

// Complete godoc
// POST /api/v1/tasks/{id}/complete
// Convenience endpoint — no body required.
func (h *Handler) Complete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	task, err := h.svc.Complete(id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respondError(w, http.StatusNotFound, "task not found", nil)
			return
		}
		if isValidationError(err) {
			respondError(w, http.StatusBadRequest, err.Error(), nil)
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to complete task", err)
		return
	}

	respondJSON(w, http.StatusOK, task)
}

// Delete godoc
// DELETE /api/v1/tasks/{id}
// Soft-delete — the task is not physically removed.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.svc.Delete(id); err != nil {
		if errors.Is(err, ErrNotFound) {
			respondError(w, http.StatusNotFound, "task not found", nil)
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to delete task", err)
		return
	}

	w.WriteHeader(http.StatusNoContent) // 204 — success, no body
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// errorResponse is the standard error envelope returned by all endpoints.
type errorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

func respondJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func respondError(w http.ResponseWriter, status int, message string, _ error) {
	// We intentionally do not expose internal error details to the client
	// (the internal error is logged by the middleware, not returned in the body).
	respondJSON(w, status, errorResponse{
		Error:   http.StatusText(status),
		Message: message,
	})
}

func decodeJSON(r *http.Request, dst any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields() // catches typos in field names
	return dec.Decode(dst)
}

// isValidationError returns true for errors that should map to 400 Bad Request.
// Validation errors come from model.Validate() and service.validateTransition().
// They are plain errors with a user-readable message — not wrapped sentinel errors.
func isValidationError(err error) bool {
	// If it's not one of our sentinel errors, treat it as a validation message.
	return !errors.Is(err, ErrNotFound) && !errors.Is(err, ErrTagNotFound)
}
