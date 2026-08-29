package poller

import (
	"context"
	"fmt"
	"time"

	"fleetgauge/internal/backend"
)

// Defaults used when New is given a nonsensical interval or depth.
const (
	DefaultInterval = 5 * time.Second
	DefaultDepth    = 120
)

// Poller samples a fleet on an interval and writes what it sees into a Store.
//
// One Poller owns one Store. Everything above — the page, /events, /metrics —
// reads the Store; nothing above ever calls the backend for state, so there is
// exactly one place where "how often do we ask systemd" is decided.
type Poller struct {
	be       backend.Backend
	patterns []string
	interval time.Duration
	store    *Store

	// Now is the clock, injected so tests can pin sample stamps. Nil means
	// time.Now. Set it before Run; changing it on a running poller is a race.
	Now func() time.Time
}

// New returns a Poller watching patterns through be, sampling every interval
// and keeping depth samples per unit. A non-positive interval or depth falls
// back to the package defaults rather than spinning or dropping history.
func New(be backend.Backend, patterns []string, interval time.Duration, depth int) *Poller {
	if interval <= 0 {
		interval = DefaultInterval
	}
	if depth < 1 {
		depth = DefaultDepth
	}
	return &Poller{
		be:       be,
		patterns: append([]string(nil), patterns...),
		interval: interval,
		store:    NewStore(depth),
	}
}

// Store returns the history this poller writes into.
func (p *Poller) Store() *Store { return p.store }

// Interval reports the configured poll interval.
func (p *Poller) Interval() time.Duration { return p.interval }

// Patterns returns the configured unit patterns.
func (p *Poller) Patterns() []string { return append([]string(nil), p.patterns...) }

func (p *Poller) now() time.Time {
	if p.Now != nil {
		return p.Now()
	}
	return time.Now()
}

// PollOnce runs one List-then-Show cycle and records the result, returning the
// transitions that poll observed.
//
// A backend failure is recorded on the Store and returned, but the history
// already held is left untouched: stale state with an honest age beats a blank
// page. Callers in a loop are expected to keep going.
func (p *Poller) PollOnce(ctx context.Context) ([]Transition, error) {
	names, err := p.be.List(ctx, p.patterns)
	if err != nil {
		err = fmt.Errorf("list units: %w", err)
		p.store.RecordError(p.now(), err)
		return nil, err
	}

	// No matches is a legitimate answer, not a failure: record the empty poll
	// so snapshot age keeps advancing and the page can say "watching nothing".
	if len(names) == 0 {
		return p.store.Record(p.now(), nil), nil
	}

	snaps, err := p.be.Show(ctx, names)
	if err != nil {
		err = fmt.Errorf("show units: %w", err)
		p.store.RecordError(p.now(), err)
		return nil, err
	}

	return p.store.Record(p.now(), snaps), nil
}

// Run polls immediately, then every interval, until ctx is done. It returns
// ctx.Err().
//
// Backend failures never stop the loop — they are recorded on the Store and the
// next tick tries again. A monitoring tool that exits when the thing it watches
// misbehaves is worse than useless.
func (p *Poller) Run(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	_, _ = p.PollOnce(ctx)

	t := time.NewTicker(p.interval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
			_, _ = p.PollOnce(ctx)
		}
	}
}
