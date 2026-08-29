package poller

import (
	"context"
	"errors"
	"testing"
	"time"

	"fleetgauge/internal/backend/fake"
)

// This file is the planning lane's smoke test: it proves the poller pipeline
// compiles and works end to end against the fake backend. It is deliberately
// thin. The thorough suites live in ring_test.go, store_test.go, poller_test.go
// and overview_test.go — add new files there rather than editing this one.

// pinnedClock returns a clock function that advances one second per call, so
// sample stamps are ordered and reproducible without reading the wall clock.
func pinnedClock() func() time.Time {
	base := time.Date(2025, 6, 15, 3, 0, 0, 0, time.UTC)
	n := 0
	return func() time.Time {
		n++
		return base.Add(time.Duration(n) * time.Second)
	}
}

func TestSmokePollOnceRecordsFleet(t *testing.T) {
	be := fake.New()
	p := New(be, []string{"*.service"}, time.Second, 10)
	p.Now = pinnedClock()

	if _, err := p.PollOnce(context.Background()); err != nil {
		t.Fatalf("PollOnce: %v", err)
	}

	st := p.Store()
	if got := len(st.Names()); got != 12 {
		t.Errorf("Names() = %d units, want 12", got)
	}
	if _, ok := st.Latest("nginx.service"); !ok {
		t.Error("Latest(nginx.service) not found after a poll")
	}
	if got := len(st.History("nginx.service")); got != 1 {
		t.Errorf("History(nginx.service) = %d samples after one poll, want 1", got)
	}
	if polls, failures := st.Counts(); polls != 1 || failures != 0 {
		t.Errorf("Counts() = (%d, %d), want (1, 0)", polls, failures)
	}
}

func TestSmokeTransitionRecordedOnStateChange(t *testing.T) {
	be := fake.New()
	p := New(be, []string{"*.service"}, time.Second, 10)
	p.Now = pinnedClock()

	ctx := context.Background()
	if _, err := p.PollOnce(ctx); err != nil {
		t.Fatalf("first PollOnce: %v", err)
	}
	be.Tick() // flappy.service flips state
	got, err := p.PollOnce(ctx)
	if err != nil {
		t.Fatalf("second PollOnce: %v", err)
	}

	var sawFlappy bool
	for _, tr := range got {
		if tr.Unit == "flappy.service" {
			sawFlappy = true
		}
	}
	if !sawFlappy {
		t.Errorf("no transition recorded for flappy.service after a tick, got %+v", got)
	}
}

func TestSmokeRunStopsOnContextCancel(t *testing.T) {
	be := fake.New()
	p := New(be, []string{"*.service"}, 2*time.Millisecond, 10)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- p.Run(ctx) }()

	// Wait for the immediate poll before cancelling: Run returns straight away
	// on an already-dead context, so cancelling first would race the loop.
	deadline := time.Now().Add(2 * time.Second)
	for {
		if polls, _ := p.Store().Counts(); polls >= 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("Run did not complete its immediate poll")
		}
		time.Sleep(time.Millisecond)
	}
	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Run returned %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after the context was cancelled")
	}

	if polls, _ := p.Store().Counts(); polls < 1 {
		t.Errorf("Run recorded %d polls, want at least the immediate one", polls)
	}
}
