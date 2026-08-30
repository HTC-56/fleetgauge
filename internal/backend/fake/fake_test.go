package fake

import (
	"context"
	"testing"

	"github.com/HTC-56/fleetgauge/internal/backend"
)

// TestNewFleetCountAndListAll asserts that New() yields exactly 12 units,
// and List("*") returns all 12 names in sorted order.
func TestNewFleetCountAndListAll(t *testing.T) {
	b := New()

	names, err := b.List(context.Background(), []string{"*"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(names) != 12 {
		t.Errorf("List count = %d, want 12", len(names))
	}

	// Verify sorted order.
	for i := 1; i < len(names); i++ {
		if names[i-1] >= names[i] {
			t.Errorf("List not sorted: %q >= %q", names[i-1], names[i])
		}
	}
}

// TestListExactAndGlob asserts that List("nginx.service") returns exactly
// that one name, and List("*.service") returns all 12.
func TestListExactAndGlob(t *testing.T) {
	b := New()

	// Exact match.
	names, err := b.List(context.Background(), []string{"nginx.service"})
	if err != nil {
		t.Fatalf("List exact: %v", err)
	}
	if len(names) != 1 || names[0] != "nginx.service" {
		t.Errorf("List exact = %v, want [nginx.service]", names)
	}

	// Glob.
	names, err = b.List(context.Background(), []string{"*.service"})
	if err != nil {
		t.Fatalf("List glob: %v", err)
	}
	if len(names) != 12 {
		t.Errorf("List *.service count = %d, want 12", len(names))
	}
}

// TestShowMissingUnit asserts that Show for a name not in the fleet returns
// one snapshot with Found == false and MemoryBytes == MemoryUnknown.
func TestShowMissingUnit(t *testing.T) {
	b := New()

	snaps, err := b.Show(context.Background(), []string{"ghost.service"})
	if err != nil {
		t.Fatalf("Show missing: %v", err)
	}
	if len(snaps) != 1 {
		t.Fatalf("Show missing returned %d snapshots, want 1", len(snaps))
	}
	if snaps[0].Found {
		t.Error("Found = true, want false for missing unit")
	}
	if snaps[0].MemoryBytes != backend.MemoryUnknown {
		t.Errorf("MemoryBytes = %d, want MemoryUnknown (%d)",
			snaps[0].MemoryBytes, backend.MemoryUnknown)
	}
}

// TestShowOrdering asserts that Show returns snapshots in the same order as
// the requested names, even when passed in a different order than the fleet's.
func TestShowOrdering(t *testing.T) {
	b := New()

	// Request in non-fleet order.
	requested := []string{"worker.service", "nginx.service", "redis.service"}
	snaps, err := b.Show(context.Background(), requested)
	if err != nil {
		t.Fatalf("Show ordering: %v", err)
	}
	if len(snaps) != 3 {
		t.Fatalf("Show returned %d snapshots, want 3", len(snaps))
	}
	for i, want := range requested {
		if snaps[i].Name != want {
			t.Errorf("snap[%d].Name = %q, want %q", i, snaps[i].Name, want)
		}
	}
}

// TestRestartIncrements asserts that Restart increments NRestarts by exactly
// one and leaves the unit active.
func TestRestartIncrements(t *testing.T) {
	b := New()

	before, err := b.Show(context.Background(), []string{"nginx.service"})
	if err != nil {
		t.Fatalf("Show before: %v", err)
	}
	beforeRestarts := before[0].NRestarts

	err = b.Restart(context.Background(), "nginx.service")
	if err != nil {
		t.Fatalf("Restart: %v", err)
	}

	after, err := b.Show(context.Background(), []string{"nginx.service"})
	if err != nil {
		t.Fatalf("Show after: %v", err)
	}
	if after[0].NRestarts != beforeRestarts+1 {
		t.Errorf("NRestarts = %d, want %d", after[0].NRestarts, beforeRestarts+1)
	}
	if after[0].ActiveState != backend.StateActive {
		t.Errorf("ActiveState = %q, want active after restart", after[0].ActiveState)
	}
}

// TestExactlyOneMemoryUnknown asserts that exactly one unit in the starting
// fleet reports MemoryBytes == MemoryUnknown.
func TestExactlyOneMemoryUnknown(t *testing.T) {
	b := New()

	names, err := b.List(context.Background(), []string{"*"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	snaps, err := b.Show(context.Background(), names)
	if err != nil {
		t.Fatalf("Show: %v", err)
	}

	unknown := 0
	for _, s := range snaps {
		if s.MemoryBytes == backend.MemoryUnknown {
			unknown++
		}
	}
	if unknown != 1 {
		t.Errorf("MemoryUnknown count = %d, want 1", unknown)
	}
}

// TestTickDeterminism asserts that two fresh fakes, ticked the same number of
// times, report identical snapshots — Tick is fully deterministic.
func TestTickDeterminism(t *testing.T) {
	a := New()
	b := New()

	for i := 0; i < 10; i++ {
		a.Tick()
		b.Tick()
	}

	names, err := a.List(context.Background(), []string{"*"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	snapsA, err := a.Show(context.Background(), names)
	if err != nil {
		t.Fatalf("Show A: %v", err)
	}

	snapsB, err := b.Show(context.Background(), names)
	if err != nil {
		t.Fatalf("Show B: %v", err)
	}

	if len(snapsA) != len(snapsB) {
		t.Fatalf("snapshot counts differ: A=%d, B=%d", len(snapsA), len(snapsB))
	}

	for i := range snapsA {
		aSnap := snapsA[i]
		bSnap := snapsB[i]
		if aSnap.ActiveState != bSnap.ActiveState ||
			aSnap.SubState != bSnap.SubState ||
			aSnap.NRestarts != bSnap.NRestarts ||
			aSnap.MemoryBytes != bSnap.MemoryBytes {
			t.Errorf("After %d ticks, snap[%d] (%s) differs: A={%v/%v/%d/%d} B={%v/%v/%d/%d}",
				10, i, aSnap.Name,
				aSnap.ActiveState, aSnap.SubState, aSnap.NRestarts, aSnap.MemoryBytes,
				bSnap.ActiveState, bSnap.SubState, bSnap.NRestarts, bSnap.MemoryBytes)
		}
	}
}

// TestFlappyEvenOdd asserts that after an even number of ticks flappy.service
// returns to its starting active/running state, while after an odd number it
// is failed.
func TestFlappyEvenOdd(t *testing.T) {
	b := New()

	// Start state: active/running.
	before, err := b.Show(context.Background(), []string{"flappy.service"})
	if err != nil {
		t.Fatalf("Show: %v", err)
	}
	if !before[0].Found || before[0].ActiveState != backend.StateActive {
		t.Fatal("flappy should start active")
	}

	// Two ticks (even) → back to active.
	b.Tick()
	b.Tick()
	two, err := b.Show(context.Background(), []string{"flappy.service"})
	if err != nil {
		t.Fatalf("Show after 2: %v", err)
	}
	if two[0].ActiveState != backend.StateActive {
		t.Errorf("After 2 ticks: ActiveState = %q, want active", two[0].ActiveState)
	}
	if two[0].SubState != "running" {
		t.Errorf("After 2 ticks: SubState = %q, want running", two[0].SubState)
	}

	// Three ticks (odd) → failed.
	b.Tick()
	three, err := b.Show(context.Background(), []string{"flappy.service"})
	if err != nil {
		t.Fatalf("Show after 3: %v", err)
	}
	if three[0].ActiveState != backend.StateFailed {
		t.Errorf("After 3 ticks: ActiveState = %q, want failed", three[0].ActiveState)
	}
}

// TestFlappyRestartsGrows asserts that flappy.service's NRestarts after 10
// ticks is greater than after 2 ticks — the counter actually increments.
func TestFlappyRestartsGrows(t *testing.T) {
	a := New()
	b := New()

	// Tick both to 2.
	for i := 0; i < 2; i++ {
		a.Tick()
		b.Tick()
	}

	two, err := a.Show(context.Background(), []string{"flappy.service"})
	if err != nil {
		t.Fatalf("Show after 2: %v", err)
	}

	// Tick both to 10.
	for i := 2; i < 10; i++ {
		a.Tick()
		b.Tick()
	}

	ten, err := a.Show(context.Background(), []string{"flappy.service"})
	if err != nil {
		t.Fatalf("Show after 10: %v", err)
	}

	if ten[0].NRestarts <= two[0].NRestarts {
		t.Errorf("NRestarts: after 10 ticks (%d) <= after 2 ticks (%d), want >",
			ten[0].NRestarts, two[0].NRestarts)
	}
}

// TestWedgedStaysFailed asserts that wedged.service remains failed after 10
// ticks — it never recovers.
func TestWedgedStaysFailed(t *testing.T) {
	b := New()

	for i := 0; i < 10; i++ {
		b.Tick()
	}

	snaps, err := b.Show(context.Background(), []string{"wedged.service"})
	if err != nil {
		t.Fatalf("Show: %v", err)
	}

	if snaps[0].ActiveState != backend.StateFailed {
		t.Errorf("ActiveState = %q, want failed after 10 ticks", snaps[0].ActiveState)
	}
	if snaps[0].SubState != "failed" {
		t.Errorf("SubState = %q, want failed after 10 ticks", snaps[0].SubState)
	}
}

// TestMemoryUnknownStays asserts that the unit with MemoryUnknown memory
// (monitor.service) still reports MemoryUnknown after 10 ticks — drift
// never turns "unknown" into a number.
func TestMemoryUnknownStays(t *testing.T) {
	b := New()

	for i := 0; i < 10; i++ {
		b.Tick()
	}

	snaps, err := b.Show(context.Background(), []string{"monitor.service"})
	if err != nil {
		t.Fatalf("Show: %v", err)
	}

	if snaps[0].MemoryBytes != backend.MemoryUnknown {
		t.Errorf("MemoryBytes = %d, want MemoryUnknown after 10 ticks",
			snaps[0].MemoryBytes)
	}
}
