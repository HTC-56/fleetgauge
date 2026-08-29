package poller

import (
	"time"

	"fleetgauge/internal/backend"
)

// UnitView is one row of the fleet overview: current state plus the history
// summary that neither the page nor /metrics need to recompute from raw samples.
type UnitView struct {
	Name        string
	Found       bool
	ActiveState backend.ActiveState
	SubState    string
	NRestarts   int
	MemoryBytes int64
	Uptime      time.Duration
	ObservedAt  time.Time
	Samples     int
	Transitions int
}

// Overview returns one UnitView per name in the store, in sorted order, built
// only from the Store's exported accessors (Names, Latest, History,
// Transitions). It takes no lock of its own — those accessors already lock.
func (s *Store) Overview(now time.Time) []UnitView {
	names := s.Names()
	out := make([]UnitView, 0, len(names))

	// Collect all transitions once so we can count per-unit without re-locking.
	allTrs := s.Transitions()
	trCount := make(map[string]int, len(allTrs))
	for _, tr := range allTrs {
		trCount[tr.Unit]++
	}

	for _, name := range names {
		snap, ok := s.Latest(name)
		v := UnitView{
			Name:        name,
			Samples:     len(s.History(name)),
			Transitions: trCount[name],
		}
		if !ok {
			v.Found = false
			out = append(out, v)
			continue
		}
		v.Found = snap.Snap.Found
		v.ActiveState = snap.Snap.ActiveState
		v.SubState = snap.Snap.SubState
		v.NRestarts = snap.Snap.NRestarts
		v.MemoryBytes = snap.Snap.MemoryBytes
		v.Uptime = snap.Snap.Uptime(now)
		v.ObservedAt = snap.At
		out = append(out, v)
	}

	return out
}
