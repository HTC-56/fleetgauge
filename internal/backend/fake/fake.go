// Package fake is a static synthetic fleet for tests and the -demo binary.
// It implements backend.Backend without systemd, so the test suite and
// -demo need no host state.
package fake

import (
	"context"
	"fmt"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	"fleetgauge/internal/backend"
)

// Backend holds a fixed fleet of synthetic units and their mutable state.
// It is safe for concurrent use: all access is guarded by a mutex.
type Backend struct {
	mu       sync.Mutex
	units    map[string]*unit
	baseTime time.Time
	step     int // deterministic tick counter — no rand, no clock
}

type unit struct {
	name        string
	activeState backend.ActiveState
	subState    string
	loadState   string
	nRestarts   int
	memoryBytes int64
	startedAt   time.Time
}

// New returns a Backend with 12 synthetic units so the page and tests
// exercise every code path without systemd or exec.
func New() *Backend {
	base := time.Date(2025, 6, 15, 3, 0, 0, 0, time.UTC)

	b := &Backend{
		units:    make(map[string]*unit),
		baseTime: base,
	}

	// fleet defines the 12-unit synthetic fleet. Each entry is
	// (name, activeState, subState, nRestarts, memoryBytes).
	// memoryBytes=0 means use MemoryUnknown (one unit only).
	fleet := []struct {
		name        string
		activeState backend.ActiveState
		subState    string
		nRestarts   int
		memoryBytes int64 // 0 → MemoryUnknown
	}{
		{"nginx.service", backend.StateActive, "running", 0, 48 << 20},
		{"postgres.service", backend.StateActive, "running", 0, 120 << 20},
		{"redis.service", backend.StateActive, "running", 0, 64 << 20},
		{"worker.service", backend.StateActive, "running", 2, 32 << 20},
		{"scheduler.service", backend.StateActive, "running", 1, 24 << 20},
		{"flappy.service", backend.StateActive, "running", 0, 16 << 20},
		{"wedged.service", backend.StateFailed, "failed", 0, 8 << 20},
		{"api.service", backend.StateActive, "running", 0, 56 << 20},
		{"gateway.service", backend.StateActive, "running", 0, 40 << 20},
		{"monitor.service", backend.StateActive, "running", 0, backend.MemoryUnknown},
		{"logger.service", backend.StateActive, "running", 0, 20 << 20},
		{"metrics.service", backend.StateActive, "running", 0, 28 << 20},
	}

	for _, entry := range fleet {
		mb := entry.memoryBytes
		if mb == 0 {
			mb = backend.MemoryUnknown
		}
		b.units[entry.name] = &unit{
			name:        entry.name,
			activeState: entry.activeState,
			subState:    entry.subState,
			loadState:   "loaded",
			nRestarts:   entry.nRestarts,
			memoryBytes: mb,
			startedAt:   base.Add(-2 * time.Hour),
		}
	}

	return b
}

// compile-time interface check — a signature drift is a build error.
var _ backend.Backend = (*Backend)(nil)

// List expands patterns into concrete unit names. Exact names pass
// through unchanged; glob patterns use path.Match against the fleet.
// Results are sorted and deduplicated.
func (b *Backend) List(_ context.Context, patterns []string) ([]string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	seen := make(map[string]struct{})

	for _, p := range patterns {
		if p == "" {
			continue
		}
		if !strings.ContainsAny(p, "*?[") {
			seen[p] = struct{}{}
			continue
		}
		for name := range b.units {
			matched, err := path.Match(p, name)
			if err != nil {
				continue
			}
			if matched {
				seen[name] = struct{}{}
			}
		}
	}

	names := make([]string, 0, len(seen))
	for n := range seen {
		names = append(names, n)
	}
	sort.Strings(names)
	return names, nil
}

// Show returns one snapshot per requested name, in request order.
// A name not in the fleet returns Found=false with StateUnknown.
func (b *Backend) Show(_ context.Context, names []string) ([]backend.UnitSnapshot, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(names) == 0 {
		return nil, nil
	}

	snaps := make([]backend.UnitSnapshot, len(names))
	for i, name := range names {
		if u, ok := b.units[name]; ok {
			snaps[i] = backend.UnitSnapshot{
				Name:        u.name,
				Found:       true,
				ActiveState: u.activeState,
				SubState:    u.subState,
				LoadState:   u.loadState,
				NRestarts:   u.nRestarts,
				MemoryBytes: u.memoryBytes,
				StartedAt:   u.startedAt,
			}
		} else {
			snaps[i] = backend.UnitSnapshot{
				Name:        name,
				Found:       false,
				ActiveState: backend.StateUnknown,
				MemoryBytes: backend.MemoryUnknown,
			}
		}
	}
	return snaps, nil
}

// JournalTail returns up to n synthetic log lines for the unit.
func (b *Backend) JournalTail(_ context.Context, name string, n int) ([]string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if n <= 0 {
		return nil, nil
	}

	lines := make([]string, 0, n)
	for i := 0; i < n; i++ {
		lines = append(lines, fmt.Sprintf("[%s] log line %d for %s",
			b.baseTime.Add(time.Duration(i+1)*time.Minute).Format(time.RFC3339), i+1, name))
	}
	return lines, nil
}

// Restart sets the unit active/running and increments NRestarts.
func (b *Backend) Restart(_ context.Context, name string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	u, ok := b.units[name]
	if !ok {
		return fmt.Errorf("unit %q not found", name)
	}

	u.activeState = backend.StateActive
	u.subState = "running"
	u.nRestarts++

	return nil
}

// Tick advances the synthetic fleet by one deterministic step.
//
// flappy.service alternates between active/running and failed/failed on each
// tick; when it returns to active its NRestarts increments by one.
// wedged.service stays failed across every tick.
// Active units drift their MemoryBytes by a small amount each tick while
// staying positive. MemoryUnknown units keep MemoryUnknown — drift never
// turns "unknown" into a number.
//
// Tick is fully deterministic: no math/rand, no wall-clock reads.
// Everything derives from the internal step counter on the Backend struct.
func (b *Backend) Tick() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.step++

	for name, u := range b.units {
		switch name {
		case "flappy.service":
			// Alternate active/running ↔ failed/failed each tick.
			// When returning to active, bump NRestarts.
			if u.activeState == backend.StateActive {
				u.activeState = backend.StateFailed
				u.subState = "failed"
			} else {
				u.activeState = backend.StateActive
				u.subState = "running"
				u.nRestarts++
			}
		case "wedged.service":
			// Wedged units never recover.
			u.activeState = backend.StateFailed
			u.subState = "failed"
		default:
			// Drift MemoryBytes for active units; leave failed units untouched.
			if u.activeState == backend.StateActive {
				if u.memoryBytes == backend.MemoryUnknown {
					// Never turn "unknown" into a concrete number.
					continue
				}
				// Drift by ±1 % of base, clamped to stay positive.
				drift := (b.step%2)*2 - 1 // alternates -1, +1, -1, +1, …
				base := int64(1 << 20)    // 1 MiB
				d := int64(drift) * base / 100
				nb := u.memoryBytes + d
				if nb < 1<<10 {
					// Clamp to 1 KiB so memory never goes non-positive.
					nb = 1 << 10
				}
				u.memoryBytes = nb
			}
		}
	}
}
