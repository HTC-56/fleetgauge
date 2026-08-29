package fake

import (
	"context"
	"testing"

	"fleetgauge/internal/backend"
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
