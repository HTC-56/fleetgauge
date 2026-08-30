package poller

import (
	"errors"
	"testing"
	"time"

	"github.com/HTC-56/fleetgauge/internal/backend"
)

// pinnedClock is defined in poller_smoke_test.go — reuse it here.

func TestStoreFreshReportsDefaults(t *testing.T) {
	st := NewStore(5)

	if got := st.Depth(); got != 5 {
		t.Errorf("Depth() = %d, want 5", got)
	}

	if got := st.Names(); len(got) != 0 {
		t.Errorf("Names() = %v, want empty, got %d entries", got, len(got))
	}

	if _, ok := st.Latest("anything.service"); ok {
		t.Error("Latest(anything.service) = true on a fresh store, want false")
	}

	if st.History("anything.service") != nil {
		t.Error("History(anything.service) != nil on a fresh store, want nil")
	}
}

func TestStoreOneRecordYieldsNamesAndCounts(t *testing.T) {
	st := NewStore(5)
	now := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)

	snaps := []backend.UnitSnapshot{
		{Name: "beta.service", ActiveState: backend.StateActive},
		{Name: "alpha.service", ActiveState: backend.StateInactive},
	}

	st.Record(now, snaps)

	names := st.Names()
	if len(names) != 2 {
		t.Fatalf("Names() length = %d, want 2", len(names))
	}
	if names[0] != "alpha.service" || names[1] != "beta.service" {
		t.Errorf("Names() = %v, want sorted [alpha beta]", names)
	}

	if got := st.History("alpha.service"); len(got) != 1 {
		t.Errorf("History(alpha) samples = %d, want 1", len(got))
	}
	if got := st.History("beta.service"); len(got) != 1 {
		t.Errorf("History(beta) samples = %d, want 1", len(got))
	}

	polls, failures := st.Counts()
	if polls != 1 || failures != 0 {
		t.Errorf("Counts() = (%d, %d), want (1, 0)", polls, failures)
	}
}

func TestStoreTransitionOnlyOnStateChange(t *testing.T) {
	st := NewStore(5)
	base := time.Date(2025, 6, 15, 3, 0, 0, 0, time.UTC)
	sameState := backend.StateActive

	// First record: no transitions ever fire for a new unit.
	trs := st.Record(base, []backend.UnitSnapshot{
		{Name: "foo.service", ActiveState: sameState},
	})
	if len(trs) != 0 {
		t.Errorf("first Record: %d transitions, want 0", len(trs))
	}

	// Second record with a different state: one transition.
	trs = st.Record(base.Add(time.Second), []backend.UnitSnapshot{
		{Name: "foo.service", ActiveState: backend.StateFailed},
	})
	if len(trs) != 1 {
		t.Fatalf("second Record: %d transitions, want 1", len(trs))
	}
	tr := trs[0]
	if tr.Unit != "foo.service" {
		t.Errorf("transition Unit = %q, want foo.service", tr.Unit)
	}
	if tr.From != backend.StateActive {
		t.Errorf("transition From = %q, want active", tr.From)
	}
	if tr.To != backend.StateFailed {
		t.Errorf("transition To = %q, want failed", tr.To)
	}
	if !tr.At.Equal(base.Add(time.Second)) {
		t.Errorf("transition At = %v, want %v", tr.At, base.Add(time.Second))
	}

	// Third record at the same state: no transition.
	trs = st.Record(base.Add(2*time.Second), []backend.UnitSnapshot{
		{Name: "foo.service", ActiveState: backend.StateFailed},
	})
	if len(trs) != 0 {
		t.Errorf("third Record: %d transitions, want 0", len(trs))
	}
}

func TestStoreDepthCapsHistoryOldestFirst(t *testing.T) {
	st := NewStore(3)
	unit := "bar.service"

	for i := 0; i < 5; i++ {
		st.Record(time.Date(2025, 6, 15, 0, 0, i+1, 0, time.UTC), []backend.UnitSnapshot{
			{Name: unit, ActiveState: backend.StateActive},
		})
	}

	samples := st.History(unit)
	if len(samples) != 3 {
		t.Fatalf("History length = %d, want 3", len(samples))
	}

	// Oldest first, increasing At.
	for i := 1; i < len(samples); i++ {
		if !samples[i].At.After(samples[i-1].At) {
			t.Errorf("sample[%d].At = %v not after sample[%d].At = %v",
				i, samples[i].At, i-1, samples[i-1].At)
		}
	}
}

func TestStoreRecordError(t *testing.T) {
	st := NewStore(5)
	now := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)

	// Record one successful poll to establish history.
	st.Record(now, []backend.UnitSnapshot{
		{Name: "ok.service", ActiveState: backend.StateActive},
	})

	// RecordError: bumps failure count, leaves history untouched.
	st.RecordError(now.Add(time.Second), errors.New("connection reset"))

	failures := 0
	_, failures = st.Counts()
	if failures != 1 {
		t.Errorf("failure count = %d, want 1", failures)
	}

	if got := st.History("ok.service"); len(got) != 1 {
		t.Errorf("history still has %d samples after error, want 1", len(got))
	}

	// LastPoll's error is non-nil.
	_, err := st.LastPoll()
	if err == nil {
		t.Error("LastPoll error = nil after RecordError, want non-nil")
	}

	// A following successful Record clears the error.
	st.Record(now.Add(2*time.Second), []backend.UnitSnapshot{
		{Name: "ok.service", ActiveState: backend.StateActive},
	})
	_, err = st.LastPoll()
	if err != nil {
		t.Errorf("LastPoll error after successful Record = %v, want nil", err)
	}
}

func TestStoreSnapshotAge(t *testing.T) {
	st := NewStore(5)

	// Before anything: zero.
	if got := st.SnapshotAge(time.Now()); got != 0 {
		t.Errorf("SnapshotAge before Record = %v, want 0", got)
	}

	now := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)
	st.Record(now, []backend.UnitSnapshot{
		{Name: "snap.service", ActiveState: backend.StateActive},
	})

	age := st.SnapshotAge(now.Add(30 * time.Second))
	want := 30 * time.Second
	if age != want {
		t.Errorf("SnapshotAge = %v, want %v", age, want)
	}
}
