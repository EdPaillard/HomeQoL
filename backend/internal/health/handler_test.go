package health_test

import (
	"encoding/json"
	"homeqol/internal/health"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLivenessHandler_returns200(t *testing.T) {
	h := health.New("test-service", "0.0.1")

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	h.LivenessHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body health.Response
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body.Status != "ok" {
		t.Errorf("expected status ok, got %q", body.Status)
	}
}

func TestReadinessHandler_allChecksPassing(t *testing.T) {
	h := health.New("test-service", "0.0.1")
	h.AddCheck("database", func() string { return "" }) // "" = healthy

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rec := httptest.NewRecorder()

	h.ReadinessHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestReadinessHandler_failingCheckReturns503(t *testing.T) {
	h := health.New("test-service", "0.0.1")
	h.AddCheck("database", func() string { return "connection refused" })

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rec := httptest.NewRecorder()

	h.ReadinessHandler(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}

	var body health.Response
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body.Status != "degraded" {
		t.Errorf("expected status degraded, got %q", body.Status)
	}
}
