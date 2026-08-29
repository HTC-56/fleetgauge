package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"fleetgauge/internal/backend/fake"
	"fleetgauge/internal/ledger"
	"fleetgauge/internal/poller"
)

// stubLedger is a thread-safe in-memory ledger for tests.
//
// It records entries when fail is false and returns an error when fail is true.
type stubLedger struct {
	mu      sync.Mutex
	entries []ledger.Entry
	fail    bool
}

// Append records the entry or returns an error when fail is set.
func (s *stubLedger) Append(e ledger.Entry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fail {
		return errors.New("ledger: simulated failure")
	}
	s.entries = append(s.entries, e)
	return nil
}

// newRestartServer builds a polled fake fleet and a Server with restart
// enabled for nginx.service. The caller supplies bearerToken, allowRestart,
// and ledger. The backend is returned so a test can inspect restart counts
// through be.Show.
func newRestartServer(t *testing.T, bearerToken string, allowRestart map[string]bool, appender Appender) (*Server, *fake.Backend) {
	t.Helper()

	be := fake.New()
	p := poller.New(be, []string{"*.service"}, time.Second, 10)
	p.Now = pinnedClock()

	_, err := p.PollOnce(context.Background())
	if err != nil {
		t.Fatalf("PollOnce: %v", err)
	}

	srv := New(Options{
		Store:        p.Store(),
		Backend:      be,
		Now:          pinnedClock(),
		BearerToken:  bearerToken,
		Ledger:       appender,
		AllowRestart: allowRestart,
	})
	t.Cleanup(srv.Close)

	return srv, be
}

// TestRestart503Unconfigured asserts that a server without a bearer token
// answers 503 to POST /units/nginx.service/restart, even when the request
// carries a valid-looking Authorization header.
func TestRestart503Unconfigured(t *testing.T) {
	srv, _ := newRestartServer(t, "", map[string]bool{"nginx.service": true}, &stubLedger{})

	req := httptest.NewRequest(http.MethodPost, "/units/nginx.service/restart", nil)
	req.Header.Set("Authorization", "Bearer some-token")
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if got := rec.Code; got != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", got, http.StatusServiceUnavailable)
	}
}

// TestRestart401BadToken asserts that with a token configured, requests
// without Authorization or with the wrong token are 401.
func TestRestart401BadToken(t *testing.T) {
	srv, _ := newRestartServer(t, "secret", map[string]bool{"nginx.service": true}, &stubLedger{})

	// No Authorization header → 401.
	req := httptest.NewRequest(http.MethodPost, "/units/nginx.service/restart", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if got := rec.Code; got != http.StatusUnauthorized {
		t.Errorf("no auth: status = %d, want %d", got, http.StatusUnauthorized)
	}

	// Wrong token → 401.
	req = httptest.NewRequest(http.MethodPost, "/units/nginx.service/restart", nil)
	req.Header.Set("Authorization", "Bearer wrong")
	rec = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if got := rec.Code; got != http.StatusUnauthorized {
		t.Errorf("wrong token: status = %d, want %d", got, http.StatusUnauthorized)
	}
}

// TestRestart403NotOptedIn asserts that a correct token aimed at a unit
// that has not opted in is 403, and the stub ledger recorded nothing.
func TestRestart403NotOptedIn(t *testing.T) {
	sl := &stubLedger{}
	srv, _ := newRestartServer(t, "secret", map[string]bool{"nginx.service": true}, sl)

	req := httptest.NewRequest(http.MethodPost, "/units/redis.service/restart", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if got := rec.Code; got != http.StatusForbidden {
		t.Errorf("status = %d, want %d", got, http.StatusForbidden)
	}

	sl.mu.Lock()
	defer sl.mu.Unlock()
	if n := len(sl.entries); n != 0 {
		t.Errorf("ledger entries = %d, want 0", n)
	}
}

// TestRestart200Ok asserts that a correct token aimed at an opted-in unit
// returns 200 with result "ok" and writes two ledger lines: requested, then ok.
func TestRestart200Ok(t *testing.T) {
	sl := &stubLedger{}
	srv, _ := newRestartServer(t, "secret", map[string]bool{"nginx.service": true}, sl)

	req := httptest.NewRequest(http.MethodPost, "/units/nginx.service/restart", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if got := rec.Code; got != http.StatusOK {
		t.Fatalf("status = %d, want %d", got, http.StatusOK)
	}

	var body RestartJSON
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Result != "ok" {
		t.Errorf("result = %q, want %q", body.Result, "ok")
	}

	sl.mu.Lock()
	defer sl.mu.Unlock()
	if n := len(sl.entries); n != 2 {
		t.Fatalf("ledger entries = %d, want 2", n)
	}
	if sl.entries[0].Result != "requested" {
		t.Errorf("entry[0].result = %q, want %q", sl.entries[0].Result, "requested")
	}
	if sl.entries[1].Result != "ok" {
		t.Errorf("entry[1].result = %q, want %q", sl.entries[1].Result, "ok")
	}
}

// TestRestartLedgerFailRefuses asserts that when the stub ledger fails,
// the request is 500 and the fake backend's restart count did not change.
func TestRestartLedgerFailRefuses(t *testing.T) {
	sl := &stubLedger{fail: true}
	srv, be := newRestartServer(t, "secret", map[string]bool{"nginx.service": true}, sl)

	// Read the restart count before the request.
	snaps, err := be.Show(context.Background(), []string{"nginx.service"})
	if err != nil {
		t.Fatalf("Show before: %v", err)
	}
	before := snaps[0].NRestarts

	req := httptest.NewRequest(http.MethodPost, "/units/nginx.service/restart", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if got := rec.Code; got != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", got, http.StatusInternalServerError)
	}

	// The backend restart count must be unchanged — the action was refused.
	snaps, err = be.Show(context.Background(), []string{"nginx.service"})
	if err != nil {
		t.Fatalf("Show after: %v", err)
	}
	after := snaps[0].NRestarts
	if after != before {
		t.Errorf("restart count changed from %d to %d — action should have been refused", before, after)
	}
}

// TestRestartGet405 asserts that GET /units/{name}/restart is 405.
// The route is POST-only.
func TestRestartGet405(t *testing.T) {
	srv, _ := newRestartServer(t, "secret", map[string]bool{"nginx.service": true}, &stubLedger{})

	req := httptest.NewRequest(http.MethodGet, "/units/nginx.service/restart", nil)
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if got := rec.Code; got != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", got, http.StatusMethodNotAllowed)
	}
}
