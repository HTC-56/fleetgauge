package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestLogRequestsOneRecord asserts that a handler that returns normally
// produces exactly one log record whose method and path match the request.
func TestLogRequestsOneRecord(t *testing.T) {
	var buf strings.Builder
	log := slog.New(slog.NewJSONHandler(&buf, nil))

	mux := http.NewServeMux()
	mux.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := LogRequests(log, mux)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	var entry map[string]any
	if err := json.Unmarshal([]byte(buf.String()), &entry); err != nil {
		t.Fatalf("log output is not valid JSON: %v", err)
	}

	if entry["level"] != "INFO" {
		t.Errorf("log level = %v, want INFO", entry["level"])
	}

	if entry["method"] != "GET" {
		t.Errorf("method = %v, want GET", entry["method"])
	}
	if entry["path"] != "/test" {
		t.Errorf("path = %v, want /test", entry["path"])
	}
}

// TestLogRequestsWriteHeaderCapturesStatus asserts that a handler that
// calls WriteHeader(404) logs status 404.
func TestLogRequestsWriteHeaderCapturesStatus(t *testing.T) {
	var buf strings.Builder
	log := slog.New(slog.NewJSONHandler(&buf, nil))

	handler := LogRequests(log, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	var entry map[string]any
	if err := json.Unmarshal([]byte(buf.String()), &entry); err != nil {
		t.Fatalf("log output is not valid JSON: %v", err)
	}

	if entry["status"] != float64(404) {
		t.Errorf("status = %v, want 404", entry["status"])
	}
}

// TestLogRequestsImplicitStatus asserts that a handler that writes a body
// without calling WriteHeader logs status 200.
func TestLogRequestsImplicitStatus(t *testing.T) {
	var buf strings.Builder
	log := slog.New(slog.NewJSONHandler(&buf, nil))

	handler := LogRequests(log, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/hello", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	var entry map[string]any
	if err := json.Unmarshal([]byte(buf.String()), &entry); err != nil {
		t.Fatalf("log output is not valid JSON: %v", err)
	}

	if entry["status"] != float64(200) {
		t.Errorf("status = %v, want 200", entry["status"])
	}
}

// TestLogRequestsFlusher asserts that the wrapper satisfies http.Flusher:
// inside the wrapped handler, a type assertion of the ResponseWriter to
// http.Flusher succeeds, and calling Flush() does not panic.
func TestLogRequestsFlusher(t *testing.T) {
	var buf strings.Builder
	log := slog.New(slog.NewJSONHandler(&buf, nil))

	var flusherReceived bool

	handler := LogRequests(log, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Error("ResponseWriter does not implement http.Flusher")
			return
		}
		flusherReceived = true
		flusher.Flush() // should not panic
	}))

	req := httptest.NewRequest(http.MethodGet, "/stream", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !flusherReceived {
		t.Fatal("handler never received a ResponseWriter that implements http.Flusher")
	}
}

// TestLogRequestBodyUnchanged asserts that the response body the client
// receives is unchanged by the wrapping.
func TestLogRequestBodyUnchanged(t *testing.T) {
	var buf strings.Builder
	log := slog.New(slog.NewJSONHandler(&buf, nil))

	wantBody := "hello, world"

	handler := LogRequests(log, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(wantBody))
	}))

	req := httptest.NewRequest(http.MethodPost, "/echo", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Body.String() != wantBody {
		t.Errorf("body = %q, want %q", rec.Body.String(), wantBody)
	}
}
