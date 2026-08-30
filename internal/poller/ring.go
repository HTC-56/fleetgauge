// Package poller turns a backend into history.
//
// It samples the configured fleet on an interval and writes each observation
// into a bounded, per-unit ring buffer. There is no database: SPEC.md feature 3
// pre-registers that the ring buffer IS the history and that its depth is
// config. Nothing here knows systemd exists — the poller speaks only
// backend.Backend and the types in internal/backend.
package poller

import (
	"time"

	"github.com/HTC-56/fleetgauge/internal/backend"
)

// Sample is one observation of one unit, stamped with the time the poller took
// it. The stamp is the poller's clock, not systemd's: every unit in the same
// poll shares one At, which is what makes a fleet-wide timeline line up.
type Sample struct {
	At   time.Time
	Snap backend.UnitSnapshot
}

// ring is a fixed-capacity history buffer. Adding to a full ring overwrites the
// oldest entry, so memory is bounded by config depth no matter how long the
// process runs.
//
// A ring carries no lock of its own: Store owns the mutex and is the only thing
// that touches one.
type ring struct {
	buf   []Sample
	next  int // index the next add writes to
	count int // entries held, saturating at len(buf)
}

// newRing returns a ring holding at most n samples. n below 1 is raised to 1 —
// a zero-capacity history would silently discard every sample it was given.
func newRing(n int) *ring {
	if n < 1 {
		n = 1
	}
	return &ring{buf: make([]Sample, n)}
}

// capacity reports the maximum number of samples the ring holds. (Not named
// cap: that would shadow the builtin inside the method set.)
func (r *ring) capacity() int { return len(r.buf) }

// length reports how many samples are currently held.
func (r *ring) length() int { return r.count }

// add appends a sample, overwriting the oldest once the ring is full.
func (r *ring) add(s Sample) {
	r.buf[r.next] = s
	r.next = (r.next + 1) % len(r.buf)
	if r.count < len(r.buf) {
		r.count++
	}
}

// all returns the held samples oldest first, in a fresh slice the caller owns.
func (r *ring) all() []Sample {
	out := make([]Sample, 0, r.count)
	start := r.next - r.count
	if start < 0 {
		start += len(r.buf)
	}
	for i := 0; i < r.count; i++ {
		out = append(out, r.buf[(start+i)%len(r.buf)])
	}
	return out
}

// last returns the most recent sample. The bool is false when the ring is
// empty, which is how a caller tells "never observed" from "observed as zero".
func (r *ring) last() (Sample, bool) {
	if r.count == 0 {
		return Sample{}, false
	}
	i := r.next - 1
	if i < 0 {
		i += len(r.buf)
	}
	return r.buf[i], true
}
