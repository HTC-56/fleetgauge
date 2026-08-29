package poller

import (
	"context"
	"testing"
	"time"

	"fleetgauge/internal/backend"
	"fleetgauge/internal/backend/fake"
)

func TestOverviewReturnsFleetViewsInSortedOrder(t *testing.T) {
	be := fake.New()
	p := New(be, []string{"*.service"}, time.Second, 10)
	p.Now = pinnedClock()

	ctx := context.Background()
	if _, err := p.PollOnce(ctx); err != nil {
		t.Fatalf("PollOnce: %v", err)
	}

	now := p.Now()
	views := p.Store().Overview(now)
	if len(views) != 12 {
		t.Errorf("Overview = %d views, want 12", len(views))
	}

	// Verify sorted order.
	for i := 1; i < len(views); i++ {
		if views[i].Name < views[i-1].Name {
			t.Errorf("Overview[%d] = %q not after %q, want sorted",
				i, views[i].Name, views[i-1].Name)
		}
	}
}

func TestOverviewWedgedReportsFailed(t *testing.T) {
	be := fake.New()
	p := New(be, []string{"*.service"}, time.Second, 10)
	p.Now = pinnedClock()

	ctx := context.Background()
	if _, err := p.PollOnce(ctx); err != nil {
		t.Fatalf("PollOnce: %v", err)
	}

	now := p.Now()
	for _, v := range p.Store().Overview(now) {
		if v.Name == "wedged.service" && v.ActiveState != backend.StateFailed {
			t.Errorf("wedged.service ActiveState = %q, want %q",
				v.ActiveState, backend.StateFailed)
		}
	}
}

func TestOverviewFlappyRecordsTransitions(t *testing.T) {
	be := fake.New()
	p := New(be, []string{"*.service"}, time.Second, 10)
	p.Now = pinnedClock()

	ctx := context.Background()
	if _, err := p.PollOnce(ctx); err != nil {
		t.Fatalf("first PollOnce: %v", err)
	}

	be.Tick() // flappy.service flips state
	if _, err := p.PollOnce(ctx); err != nil {
		t.Fatalf("second PollOnce: %v", err)
	}

	now := p.Now()
	for _, v := range p.Store().Overview(now) {
		if v.Name == "flappy.service" && v.Transitions == 0 {
			t.Errorf("flappy.service Transitions = 0 after a tick, want > 0")
		}
	}
}

func TestOverviewMemoryUnknownIsNeverZero(t *testing.T) {
	be := fake.New()
	p := New(be, []string{"*.service"}, time.Second, 10)
	p.Now = pinnedClock()

	ctx := context.Background()
	if _, err := p.PollOnce(ctx); err != nil {
		t.Fatalf("PollOnce: %v", err)
	}

	now := p.Now()
	for _, v := range p.Store().Overview(now) {
		if v.Name == "monitor.service" && v.MemoryBytes == 0 {
			t.Errorf("monitor.service MemoryBytes = 0, want %d (MemoryUnknown)",
				backend.MemoryUnknown)
		}
	}
}

func TestOverviewSamplesNeverExceedDepth(t *testing.T) {
	be := fake.New()
	p := New(be, []string{"*.service"}, 10*time.Millisecond, 3) // depth=3
	p.Now = pinnedClock()

	ctx := context.Background()
	// Poll three times so every unit accumulates 3 samples.
	for i := 0; i < 3; i++ {
		p.PollOnce(ctx)
		be.Tick()
	}

	now := p.Now()
	views := p.Store().Overview(now)
	depth := p.Store().Depth()
	for _, v := range views {
		if v.Samples > depth {
			t.Errorf("%s Samples = %d > Depth() = %d",
				v.Name, v.Samples, depth)
		}
	}
}
