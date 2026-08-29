package poller

import (
	"testing"
	"time"

	"fleetgauge/internal/backend"
)

// pinnedClock is defined in poller_smoke_test.go — reuse, never redefine.

func sampleAt(t time.Time, name string) Sample {
	return Sample{
		At: t,
		Snap: backend.UnitSnapshot{
			Name:  name,
			Found: true,
		},
	}
}

func TestRingCapacityAndEmpty(t *testing.T) {
	r := newRing(3)

	if got := r.capacity(); got != 3 {
		t.Errorf("capacity() = %d, want 3", got)
	}
	if got := r.length(); got != 0 {
		t.Errorf("length() = %d, want 0", got)
	}
	if _, ok := r.last(); ok {
		t.Error("last() on empty ring should return false for second result")
	}
}

func TestRingAddTwoOldestFirst(t *testing.T) {
	r := newRing(5)

	t0 := time.Date(2025, 6, 15, 3, 0, 0, 0, time.UTC)
	r.add(sampleAt(t0, "alpha.service"))
	r.add(sampleAt(t0.Add(time.Second), "beta.service"))

	if got := r.length(); got != 2 {
		t.Errorf("length() = %d, want 2", got)
	}
	got := r.all()
	if len(got) != 2 {
		t.Fatalf("len(all()) = %d, want 2", len(got))
	}
	if got[0].Snap.Name != "alpha.service" {
		t.Errorf("all()[0].Name = %s, want alpha.service", got[0].Snap.Name)
	}
	if got[1].Snap.Name != "beta.service" {
		t.Errorf("all()[1].Name = %s, want beta.service", got[1].Snap.Name)
	}
}

func TestRingWrapKeepsLastThree(t *testing.T) {
	r := newRing(3)

	base := time.Date(2025, 6, 15, 3, 0, 0, 0, time.UTC)
	names := []string{"a", "b", "c", "d", "e"}
	for i, n := range names {
		r.add(sampleAt(base.Add(time.Duration(i)*time.Second), n+".service"))
	}

	if got := r.length(); got != 3 {
		t.Errorf("length() = %d, want 3", got)
	}
	if got := r.capacity(); got != 3 {
		t.Errorf("capacity() = %d, want 3", got)
	}
	got := r.all()
	if len(got) != 3 {
		t.Fatalf("len(all()) = %d, want 3", len(got))
	}
	want := []string{"c", "d", "e"}
	for i, s := range got {
		wantName := want[i] + ".service"
		if s.Snap.Name != wantName {
			t.Errorf("all()[%d].Name = %s, want %s", i, s.Snap.Name, wantName)
		}
	}
}

func TestLastBeforeAndAfterWrap(t *testing.T) {
	r := newRing(3)

	base := time.Date(2025, 6, 15, 3, 0, 0, 0, time.UTC)

	// Before wrap: last() returns the one sample we added.
	r.add(sampleAt(base, "first.service"))
	if s, ok := r.last(); !ok || s.Snap.Name != "first.service" {
		t.Errorf("last() before wrap = %+v, want first.service", s)
	}

	// Wrap past capacity.
	r.add(sampleAt(base.Add(time.Second), "second.service"))
	r.add(sampleAt(base.Add(2*time.Second), "third.service"))
	r.add(sampleAt(base.Add(3*time.Second), "fourth.service"))

	// After wrap: last() returns the most recently added.
	if s, ok := r.last(); !ok || s.Snap.Name != "fourth.service" {
		t.Errorf("last() after wrap = %+v, want fourth.service", s)
	}
}

func TestRingZeroAndNegativeCapacity(t *testing.T) {
	r0 := newRing(0)
	if got := r0.capacity(); got != 1 {
		t.Errorf("newRing(0).capacity() = %d, want 1", got)
	}
	r0.add(sampleAt(time.Now(), "ok.service")) // must not panic

	rNeg := newRing(-1)
	if got := rNeg.capacity(); got != 1 {
		t.Errorf("newRing(-1).capacity() = %d, want 1", got)
	}
	rNeg.add(sampleAt(time.Now(), "ok.service")) // must not panic
}

func TestAllReturnsFreshSlice(t *testing.T) {
	r := newRing(5)

	base := time.Date(2025, 6, 15, 3, 0, 0, 0, time.UTC)
	r.add(sampleAt(base, "alpha.service"))

	got := r.all()
	got[0].Snap.Name = "MUTATED"

	next := r.all()
	if next[0].Snap.Name != "alpha.service" {
		t.Errorf("second all()[0].Name = %s, want alpha.service (fresh slice)", next[0].Snap.Name)
	}
}
