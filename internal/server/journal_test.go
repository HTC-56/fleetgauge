package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/HTC-56/fleetgauge/internal/poller"
)

func TestJournalReturnsLines(t *testing.T) {
	srv, _, _ := newTestServer(t, 2)

	req := httptest.NewRequest(http.MethodGet, "/units/nginx.service/journal", nil)
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var body JournalJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body.Name != "nginx.service" {
		t.Errorf("name = %q, want %q", body.Name, "nginx.service")
	}
	if len(body.Lines) == 0 {
		t.Error("lines is empty, want synthetic journal lines")
	}
	if len(body.Lines) > srv.JournalLines() {
		t.Errorf("lines = %d, want at most %d", len(body.Lines), srv.JournalLines())
	}
}

func TestJournalNoBackend(t *testing.T) {
	store := poller.NewStore(10)
	srv := New(Options{Store: store})
	defer srv.Close()

	req := httptest.NewRequest(http.MethodGet, "/units/nginx.service/journal", nil)
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", rec.Code)
	}
	var body map[string]string
	json.Unmarshal(rec.Body.Bytes(), &body)
	if body["error"] == "" {
		t.Error("error field is empty, want message")
	}
}

func TestJournalContentType(t *testing.T) {
	srv, _, _ := newTestServer(t, 2)

	req := httptest.NewRequest(http.MethodGet, "/units/nginx.service/journal", nil)
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}
}

func TestJournalEmptyName(t *testing.T) {
	srv, _, _ := newTestServer(t, 2)

	// Call the handler directly — outside the mux r.PathValue("name")
	// returns "", which is exactly the empty-name case we want.
	req := httptest.NewRequest(http.MethodGet, "/units//journal", nil)
	rec := httptest.NewRecorder()

	srv.HandleJournal(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}
