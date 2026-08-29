package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"fleetgauge/internal/backend/fake"
	"fleetgauge/internal/poller"
)

// This file is the planning lane's smoke test: it proves the SSE hub, the
// fleet payload and the /events handler work end to end against the fake
// backend. It is deliberately thin. The thorough suites live in server_test.go,
// hub_test.go, log_test.go and journal_test.go — add new files there rather
// than editing this one.
//
// The helpers below are shared by every test in the package. Reuse them; a
// second definition of any of them in the same package is a compile error.

// pinnedClock returns a clock that advances one second per call, so timestamps
// are ordered and reproducible without reading the wall clock.
func pinnedClock() func() time.Time {
	base := time.Date(2025, 6, 15, 3, 0, 0, 0, time.UTC)
	n := 0
	return func() time.Time {
		n++
		return base.Add(time.Duration(n) * time.Second)
	}
}

// newTestServer returns a Server over a freshly polled synthetic fleet, plus
// the fake backend driving it so a test can Tick() and poll again. polls says
// how many times to poll; a Tick happens between consecutive polls, so two or
// more polls guarantee flappy.service has changed state at least once.
//
// The returned Server is closed automatically when the test ends.
func newTestServer(t *testing.T, polls int) (*Server, *fake.Backend, *poller.Poller) {
	t.Helper()

	be := fake.New()
	p := poller.New(be, []string{"*.service"}, time.Second, 10)
	p.Now = pinnedClock()

	for i := 0; i < polls; i++ {
		if i > 0 {
			be.Tick()
		}
		if _, err := p.PollOnce(context.Background()); err != nil {
			t.Fatalf("PollOnce %d: %v", i+1, err)
		}
	}

	srv := New(Options{
		Store:   p.Store(),
		Backend: be,
		Now:     pinnedClock(),
	})
	t.Cleanup(srv.Close)

	return srv, be, p
}

// waitFor blocks until cond returns true, failing the test after a second.
// Use it instead of a bare sleep whenever a goroutine has to get somewhere.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()

	deadline := time.Now().Add(time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(time.Millisecond)
	}
}

func TestSmokeFleetPayloadCoversTheFleet(t *testing.T) {
	srv, _, _ := newTestServer(t, 2)

	f := srv.Fleet()
	if len(f.Units) != 12 {
		t.Errorf("Fleet().Units = %d, want 12", len(f.Units))
	}
	if f.Counts.Total != 12 {
		t.Errorf("Counts.Total = %d, want 12", f.Counts.Total)
	}
	if f.Counts.Failed < 1 {
		t.Errorf("Counts.Failed = %d, want at least wedged.service", f.Counts.Failed)
	}
	if f.Polls != 2 {
		t.Errorf("Polls = %d, want 2", f.Polls)
	}

	// The payload must survive a JSON round trip: the page parses it.
	var back FleetJSON
	if err := json.Unmarshal([]byte(mustJSON(f)), &back); err != nil {
		t.Fatalf("payload does not round-trip: %v", err)
	}
	if len(back.Units) != len(f.Units) {
		t.Errorf("round-tripped %d units, want %d", len(back.Units), len(f.Units))
	}
}

func TestSmokeHubDeliversToSubscribers(t *testing.T) {
	h := NewHub()
	defer h.Close()

	ch, cancel := h.Subscribe()
	defer cancel()

	if got := h.Count(); got != 1 {
		t.Fatalf("Count() = %d after one Subscribe, want 1", got)
	}
	h.Publish(Event{Name: "fleet", Data: `{"ok":true}`})

	select {
	case ev := <-ch:
		if ev.Name != "fleet" {
			t.Errorf("event name = %q, want %q", ev.Name, "fleet")
		}
	case <-time.After(time.Second):
		t.Fatal("published event never arrived")
	}

	cancel()
	if got := h.Count(); got != 0 {
		t.Errorf("Count() = %d after cancel, want 0", got)
	}
}

func TestSmokeEventsStreamsTheFleet(t *testing.T) {
	srv, _, _ := newTestServer(t, 2)

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/events", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		defer close(done)
		srv.HandleEvents(rec, req)
	}()

	// The handler subscribes before it writes, so a live subscriber means the
	// opening payload is either written or about to be.
	waitFor(t, "the SSE subscriber to register", func() bool { return srv.Hub().Count() == 1 })
	srv.PublishOnce()
	cancel()
	<-done

	if got := rec.Header().Get("Content-Type"); got != "text/event-stream" {
		t.Errorf("Content-Type = %q, want text/event-stream", got)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "event: fleet\n") {
		t.Errorf("stream carried no fleet event:\n%s", body)
	}
	if !strings.Contains(body, "data: {") {
		t.Errorf("stream carried no JSON data line:\n%s", body)
	}
	if srv.Hub().Count() != 0 {
		t.Error("subscriber was not unregistered when the request ended")
	}
}
