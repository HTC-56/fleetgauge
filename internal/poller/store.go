package poller

import (
	"sort"
	"sync"
	"time"

	"github.com/HTC-56/fleetgauge/internal/backend"
)

// Transition is one observed change of a unit's ActiveState.
//
// A transition is recorded when a poll sees a state different from the state
// the previous poll saw. The first observation of a unit is never a transition:
// there is nothing to change from, and reporting "unknown -> active" for every
// unit at startup would fill the timeline with noise on every restart of
// fleetgauge itself.
type Transition struct {
	Unit string
	From backend.ActiveState
	To   backend.ActiveState
	At   time.Time
}

// Store is the in-memory history: a bounded ring of samples per unit, a bounded
// list of fleet-wide transitions, and the bookkeeping the page and /metrics
// need (snapshot age, poll counters, the last error).
//
// Every accessor returns copies. A reader can therefore never observe a
// half-written poll, and never mutate what the poller is holding.
// Store is safe for concurrent use.
type Store struct {
	mu sync.Mutex

	depth          int
	maxTransitions int

	units map[string]*ring
	names []string // fleet as of the last recorded poll, sorted

	transitions []Transition

	lastPoll time.Time
	lastErr  error
	polls    int
	failures int
}

// NewStore returns a Store keeping depth samples per unit. depth below 1 is
// raised to 1.
//
// The transition log is sized at four times depth, floored at 64: transitions
// are far rarer than samples, and a timeline that only reaches back as far as
// the sample history would forget the flap that explains the current state.
func NewStore(depth int) *Store {
	if depth < 1 {
		depth = 1
	}
	maxTransitions := depth * 4
	if maxTransitions < 64 {
		maxTransitions = 64
	}
	return &Store{
		depth:          depth,
		maxTransitions: maxTransitions,
		units:          make(map[string]*ring),
	}
}

// Depth reports the per-unit sample capacity.
func (s *Store) Depth() int { return s.depth }

// Record stores one poll's worth of snapshots, detects the state transitions
// against each unit's previous sample, and returns the transitions it recorded.
// now stamps every sample in the batch.
//
// Units absent from snaps keep the history they already have — they simply drop
// out of Names. A glob that stops matching must not erase what it matched.
func (s *Store) Record(now time.Time, snaps []backend.UnitSnapshot) []Transition {
	s.mu.Lock()
	defer s.mu.Unlock()

	var recorded []Transition
	names := make([]string, 0, len(snaps))

	for _, snap := range snaps {
		names = append(names, snap.Name)

		r, ok := s.units[snap.Name]
		if !ok {
			r = newRing(s.depth)
			s.units[snap.Name] = r
		}

		if prev, had := r.last(); had && prev.Snap.ActiveState != snap.ActiveState {
			t := Transition{
				Unit: snap.Name,
				From: prev.Snap.ActiveState,
				To:   snap.ActiveState,
				At:   now,
			}
			recorded = append(recorded, t)
			s.transitions = appendCapped(s.transitions, t, s.maxTransitions)
		}

		r.add(Sample{At: now, Snap: snap})
	}

	sort.Strings(names)
	s.names = names
	s.lastPoll = now
	s.lastErr = nil
	s.polls++

	return recorded
}

// RecordError notes a poll that failed. History is deliberately left intact: a
// page showing the last known state next to an honest snapshot age is more
// useful than a page that blanks itself the moment systemd hiccups.
func (s *Store) RecordError(now time.Time, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.lastErr = err
	s.failures++
	_ = now // the failure time is not history; LastPoll stays at the last good poll
}

// Names returns the fleet as of the last recorded poll, sorted.
func (s *Store) Names() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	return append([]string(nil), s.names...)
}

// History returns the samples held for name, oldest first. An unknown name
// returns nil rather than an error: the page asks about whatever it was last
// told, and a unit can leave the fleet between one request and the next.
func (s *Store) History(name string) []Sample {
	s.mu.Lock()
	defer s.mu.Unlock()

	r, ok := s.units[name]
	if !ok {
		return nil
	}
	return r.all()
}

// Latest returns the most recent sample for name. The bool is false when the
// unit has never been observed.
func (s *Store) Latest(name string) (Sample, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	r, ok := s.units[name]
	if !ok {
		return Sample{}, false
	}
	return r.last()
}

// Transitions returns the recorded transitions, oldest first.
func (s *Store) Transitions() []Transition {
	s.mu.Lock()
	defer s.mu.Unlock()

	return append([]Transition(nil), s.transitions...)
}

// LastPoll returns the time of the last successful poll — zero when nothing has
// been recorded yet — and the error from the most recent failed poll, which is
// nil whenever the last poll succeeded.
func (s *Store) LastPoll() (time.Time, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.lastPoll, s.lastErr
}

// SnapshotAge reports how long ago the last successful poll happened. It
// returns zero when nothing has been recorded yet; callers that need to tell
// "fresh" from "never" check LastPoll.
func (s *Store) SnapshotAge(now time.Time) time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.lastPoll.IsZero() {
		return 0
	}
	if d := now.Sub(s.lastPoll); d > 0 {
		return d
	}
	return 0
}

// Counts returns the number of successful and failed polls since start.
func (s *Store) Counts() (polls, failures int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.polls, s.failures
}

// appendCapped appends t and keeps at most max entries, dropping from the
// front. The copy keeps the backing array bounded instead of resliced forever.
func appendCapped(list []Transition, t Transition, max int) []Transition {
	list = append(list, t)
	if len(list) > max {
		n := copy(list, list[len(list)-max:])
		list = list[:n]
	}
	return list
}
