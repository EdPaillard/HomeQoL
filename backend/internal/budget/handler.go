package budget

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
)

type Handler struct{ svc *Service }

// NewRouter mounts all budget routes.
func NewRouter(svc *Service) http.Handler {
	h := &Handler{svc: svc}
	r := chi.NewRouter()

	// Current period (most common request — no params needed).
	r.Get("/current", h.CurrentPeriod)

	// Specific period by year/month.
	r.Get("/{year}/{month}", h.GetPeriod)
	r.Post("/periods", h.CreatePeriod)

	// Transactions.
	r.Get("/transactions", h.ListTransactions)
	r.Post("/transactions", h.CreateTransaction)
	r.Patch("/transactions/{id}/envelope", h.AssignEnvelope)

	// Accounts.
	r.Get("/accounts", h.ListAccounts)

	return r
}

// GET /api/v1/budget/current
func (h *Handler) CurrentPeriod(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	period, err := h.svc.GetPeriodWithEnvelopes(now.Year(), int(now.Month()))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			// Auto-create on first access.
			period, err = h.svc.GetOrCreateCurrentPeriod()
			if err != nil {
				respondError(w, http.StatusInternalServerError, "failed to create period", err)
				return
			}
			period, err = h.svc.GetPeriodWithEnvelopes(now.Year(), int(now.Month()))
			if err != nil {
				respondError(w, http.StatusInternalServerError, "failed to load period", err)
				return
			}
		} else {
			respondError(w, http.StatusInternalServerError, "failed to load period", err)
			return
		}
	}
	respondJSON(w, http.StatusOK, period)
}

// GET /api/v1/budget/{year}/{month}
func (h *Handler) GetPeriod(w http.ResponseWriter, r *http.Request) {
	year, month, ok := parseYearMonth(w, r)
	if !ok {
		return
	}
	period, err := h.svc.GetPeriodWithEnvelopes(year, month)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respondError(w, http.StatusNotFound, "budget period not found", nil)
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to get period", err)
		return
	}
	respondJSON(w, http.StatusOK, period)
}

// POST /api/v1/budget/periods
func (h *Handler) CreatePeriod(w http.ResponseWriter, r *http.Request) {
	var req CreatePeriodRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}
	if err := req.Validate(); err != nil {
		respondError(w, http.StatusBadRequest, err.Error(), nil)
		return
	}
	period, err := h.svc.GetOrCreatePeriod(req.Year, req.Month)
	if err != nil {
		if errors.Is(err, ErrPeriodExists) {
			respondError(w, http.StatusConflict, "period already exists", nil)
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to create period", err)
		return
	}
	respondJSON(w, http.StatusCreated, period)
}

// GET /api/v1/budget/transactions?envelope_id=&limit=&offset=
func (h *Handler) ListTransactions(w http.ResponseWriter, r *http.Request) {
	var envelopeID *string
	if e := r.URL.Query().Get("envelope_id"); e != "" {
		envelopeID = &e
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	txs, err := h.svc.ListTransactions(envelopeID, limit, offset)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list transactions", err)
		return
	}
	if txs == nil {
		txs = []Transaction{}
	}
	respondJSON(w, http.StatusOK, map[string]any{"transactions": txs, "count": len(txs)})
}

// POST /api/v1/budget/transactions
func (h *Handler) CreateTransaction(w http.ResponseWriter, r *http.Request) {
	var req CreateTransactionRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}
	tx, err := h.svc.CreateManualTransaction(req)
	if err != nil {
		if isValidation(err) {
			respondError(w, http.StatusBadRequest, err.Error(), nil)
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to create transaction", err)
		return
	}
	respondJSON(w, http.StatusCreated, tx)
}

// PATCH /api/v1/budget/transactions/{id}/envelope
func (h *Handler) AssignEnvelope(w http.ResponseWriter, r *http.Request) {
	var req AssignEnvelopeRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}
	if req.EnvelopeID == "" {
		respondError(w, http.StatusBadRequest, "envelope_id is required", nil)
		return
	}
	tx, err := h.svc.AssignEnvelope(chi.URLParam(r, "id"), req)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respondError(w, http.StatusNotFound, "transaction not found", nil)
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to assign envelope", err)
		return
	}
	respondJSON(w, http.StatusOK, tx)
}

// GET /api/v1/budget/accounts
func (h *Handler) ListAccounts(w http.ResponseWriter, r *http.Request) {
	accounts, err := h.svc.ListAccounts()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list accounts", err)
		return
	}
	if accounts == nil {
		accounts = []BankAccount{}
	}
	respondJSON(w, http.StatusOK, map[string]any{"accounts": accounts})
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

func isValidation(err error) bool {
	return !errors.Is(err, ErrNotFound) && !errors.Is(err, ErrPeriodExists) && !errors.Is(err, ErrPeriodClosed)
}

func parseYearMonth(w http.ResponseWriter, r *http.Request) (int, int, bool) {
	year, err1 := strconv.Atoi(chi.URLParam(r, "year"))
	month, err2 := strconv.Atoi(chi.URLParam(r, "month"))
	if err1 != nil || err2 != nil || month < 1 || month > 12 {
		respondError(w, http.StatusBadRequest, "year and month must be valid integers", nil)
		return 0, 0, false
	}
	return year, month, true
}
