package poller

import (
	"context"
	"fmt"
	"testing"
	"time"

	"fleetgauge/internal/backend"
	"fleetgauge/internal/backend/fake"
)

// stubErrBackend is a task-local zero-value backend whose List always
// returns an error. The other three methods return zero values successfully.
// It exercises the error path without needing a real network or systemd.
type stubErrBackend struct{}

func (s *stubErrBackend) List(_ context.Context, _ []string) ([]string, error) {
	return nil, fmt.Errorf("stub list error")
}

func (s *stubErrBackend) Show(_ context.Context, _ []string) ([]backend.UnitSnapshot, error) {
	return nil, nil
}

func (s *stubErrBackend) JournalTail(_ context.Context, _ string, _ int) ([]string, error) {
	return nil, nil
}

func (s *stubErrBackend) Restart(_ context.Context, _ string) error {
	return nil
}

// compile-time check: stubErrBackend must satisfy backend.Backend.
var _ backend.Backend = (*stubErrBackend)(nil)

// --- Assertion 1: zero interval/depth falls back to defaults ---

func TestPollerZeroIntervalAndDepth(t *testing.T) {
	be := fake.New()
	p := New(be, []string{"*.service"}, 0, 0)

	if p.Interval() != DefaultInterval {
		t.Errorf("Interval() = %v, want DefaultInterval (%v)", p.Interval(), DefaultInterval)
	}
	if p.Store().Depth() != DefaultDepth {
		t.Errorf("Store().Depth() = %d, want DefaultDepth (%d)", p.Store().Depth(), DefaultDepth)
	}
}

// --- Assertion 2: Patterns() returns a copy ---

func TestPollerPatternsReturnsCopy(t *testing.T) {
	be := fake.New()
	p := New(be, []string{"nginx.service", "*.service"}, time.Second, 5)

	// Grab two independent copies; mutating one must not leak.
	a := p.Patterns()
	a[0] = "mutated.service"

	b := p.Patterns()
	if b[0] == "mutated.service" {
		t.Errorf("Patterns() returned a shared slice: mutating a[0] leaked into b[0]")
	}
}

// --- Assertion 3: single exact pattern records exactly one unit ---

func TestPollOnceSinglePattern(t *testing.T) {
	be := fake.New()
	p := New(be, []string{"nginx.service"}, time.Second, 10)
	p.Now = pinnedClock()

	ctx := context.Background()
	_, err := p.PollOnce(ctx)
	if err != nil {
		t.Fatalf("PollOnce: %v", err)
	}

	names := p.Store().Names()
	if len(names) != 1 {
		t.Errorf("Names() = %d units, want 1", len(names))
	}
	if len(names) > 0 && names[0] != "nginx.service" {
		t.Errorf("Names()[0] = %q, want %q", names[0], "nginx.service")
	}
}

// --- Assertion 4: ten polls with ticks leave transitions; wedged stays failed ---

func TestPollerTenTicksFlappyTransitions(t *testing.T) {
	be := fake.New()
	p := New(be, []string{"*.service"}, time.Second, 10)
	p.Now = pinnedClock()

	ctx := context.Background()

	for i := 0; i < 10; i++ {
		_, _ = p.PollOnce(ctx)
		if i < 9 { // tick after each poll except the last
			be.Tick()
		}
	}

	// flappy.service should have several transitions across 10 polls + ticks.
	transitions := p.Store().Transitions()
	var flappyTrans int
	for _, tr := range transitions {
		if tr.Unit == "flappy.service" {
			flappyTrans++
		}
	}
	if flappyTrans < 3 {
		t.Errorf("flappy.service transitions = %d, want several (>= 3)", flappyTrans)
	}

	// wedged.service must still be failed after all those ticks.
	snap, ok := p.Store().Latest("wedged.service")
	if !ok {
		t.Fatal("wedged.service not in store after polling")
	}
	if snap.Snap.ActiveState != backend.StateFailed {
		t.Errorf("wedged.service state = %q, want %q", snap.Snap.ActiveState, backend.StateFailed)
	}
}

// --- Assertion 5: failing stub backend — error returned, counts zero success ---

func TestPollerStubBackendError(t *testing.T) {
	be := &stubErrBackend{}
	p := New(be, []string{"*.service"}, time.Second, 10)

	ctx := context.Background()
	_, err := p.PollOnce(ctx)
	if err == nil {
		t.Fatal("PollOnce with stub backend: want error, got nil")
	}

	polls, failures := p.Store().Counts()
	if polls != 0 {
		t.Errorf("Counts() polls = %d, want 0", polls)
	}
	if failures != 1 {
		t.Errorf("Counts() failures = %d, want 1", failures)
	}

	names := p.Store().Names()
	if len(names) != 0 {
		t.Errorf("Names() = %d units, want 0", len(names))
	}
}

// --- Assertion 6: depth 3 polled five times holds three samples ---

func TestPollerDepthCapping(t *testing.T) {
	be := fake.New()
	p := New(be, []string{"nginx.service"}, time.Second, 3)
	p.Now = pinnedClock()

	ctx := context.Background()

	for i := 0; i < 5; i++ {
		_, _ = p.PollOnce(ctx)
		be.Tick()
	}

	hist := p.Store().History("nginx.service")
	if len(hist) != 3 {
		t.Errorf("History(nginx.service) length = %d, want 3 (depth cap)", len(hist))
	}
}
