package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/HTC-56/fleetgauge/internal/poller"
)

// TestServerIndexPage asserts that GET / returns the embedded dashboard HTML
// with the correct Content-Type and the key strings the page JS expects.
func TestServerIndexPage(t *testing.T) {
	srv, _, _ := newTestServer(t, 1)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if got := rec.Code; got != http.StatusOK {
		t.Fatalf("status = %d, want %d", got, http.StatusOK)
	}

	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html/…", ct)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "<!DOCTYPE html>") {
		t.Error("body missing <!DOCTYPE html>")
	}
	if !strings.Contains(body, "EventSource") {
		t.Error("body missing EventSource")
	}
}

// TestServerMetricsText asserts that /metrics returns Prometheus exposition
// format carrying the expected unit_up gauge for nginx.service.
func TestServerMetricsText(t *testing.T) {
	srv, _, _ := newTestServer(t, 1)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if got := rec.Code; got != http.StatusOK {
		t.Fatalf("status = %d, want %d", got, http.StatusOK)
	}

	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("Content-Type = %q, want text/plain/…", ct)
	}

	body := rec.Body.String()
	if !strings.Contains(body, `fleetgauge_unit_up{unit="nginx.service"} 1`) {
		t.Error("/metrics missing nginx.service up gauge")
	}
}

// TestServerHealthzOk asserts that a polled store returns 200 with status "ok"
// and the correct unit count.
func TestServerHealthzOk(t *testing.T) {
	srv, _, _ := newTestServer(t, 1)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if got := rec.Code; got != http.StatusOK {
		t.Fatalf("status = %d, want %d", got, http.StatusOK)
	}

	var health HealthJSON
	if err := json.NewDecoder(rec.Body).Decode(&health); err != nil {
		t.Fatalf("decode health: %v", err)
	}

	if health.Status != "ok" {
		t.Errorf("status = %q, want %q", health.Status, "ok")
	}
	if health.Units != 12 {
		t.Errorf("units = %d, want 12", health.Units)
	}
}

// TestServerHealthzDegraded asserts that a store which has never been polled
// returns 503 with status "degraded".
func TestServerHealthzDegraded(t *testing.T) {
	store := poller.NewStore(5)
	srv := New(Options{Store: store})

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if got := rec.Code; got != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", got, http.StatusServiceUnavailable)
	}

	var health HealthJSON
	if err := json.NewDecoder(rec.Body).Decode(&health); err != nil {
		t.Fatalf("decode health: %v", err)
	}

	if health.Status != "degraded" {
		t.Errorf("status = %q, want %q", health.Status, "degraded")
	}
}

// TestServer404 asserts that unknown paths return 404 — the page pattern does
// not swallow them.
func TestServer404(t *testing.T) {
	srv, _, _ := newTestServer(t, 1)

	req := httptest.NewRequest(http.MethodGet, "/nope", nil)
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if got := rec.Code; got != http.StatusNotFound {
		t.Errorf("status = %d, want %d", got, http.StatusNotFound)
	}
}

// TestServer405 asserts that POST / returns 405 — the read-only surface
// accepts only GET.
func TestServer405(t *testing.T) {
	srv, _, _ := newTestServer(t, 1)

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if got := rec.Code; got != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", got, http.StatusMethodNotAllowed)
	}
}
