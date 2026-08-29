// Package backend defines the narrow seam between fleetgauge and whatever is
// actually reporting unit state. Nothing above this package knows systemd
// exists: the poller, the server, the page and /metrics all speak only the
// types declared here.
//
// Two implementations ship: internal/backend/systemd (subprocess calls to
// systemctl and journalctl) and internal/backend/fake (the synthetic fleet
// behind -demo, and the engine the whole test suite runs against).
package backend

import (
	"context"
	"time"
)

// ActiveState is the systemd high-level unit state (the ActiveState property).
// Unknown is used when a unit could not be read at all.
type ActiveState string

const (
	StateActive       ActiveState = "active"
	StateInactive     ActiveState = "inactive"
	StateFailed       ActiveState = "failed"
	StateActivating   ActiveState = "activating"
	StateDeactivating ActiveState = "deactivating"
	StateUnknown      ActiveState = "unknown"
)

// MemoryUnknown is the MemoryBytes value for a unit whose memory accounting is
// off or unavailable. systemd reports that as "[not set]" (and, on some
// versions, as uint64 max); both parse to this sentinel rather than to 0, so
// "no accounting" is never rendered or scraped as "zero bytes used".
const MemoryUnknown int64 = -1

// UnitSnapshot is one observation of one unit at one instant. It is a value:
// the poller stores copies in its ring buffer and hands copies out.
type UnitSnapshot struct {
	// Name is the full unit name, including the suffix ("nginx.service").
	Name string

	// Found reports whether the unit exists at all. A unit named in config but
	// absent on the box is reported with Found=false rather than dropped, so
	// the page can show it as missing instead of silently omitting it.
	Found bool

	ActiveState ActiveState
	SubState    string
	LoadState   string

	// NRestarts is systemd's restart counter for the unit.
	NRestarts int

	// MemoryBytes is MemoryCurrent, or MemoryUnknown when accounting is off.
	MemoryBytes int64

	// StartedAt is ExecMainStartTimestamp. Zero when the unit has never run;
	// callers must check IsZero before computing an uptime.
	StartedAt time.Time
}

// Uptime reports how long the unit has been running as of now, or zero if the
// unit has never started or is not currently active.
func (u UnitSnapshot) Uptime(now time.Time) time.Duration {
	if u.StartedAt.IsZero() || u.ActiveState != StateActive {
		return 0
	}
	if d := now.Sub(u.StartedAt); d > 0 {
		return d
	}
	return 0
}

// Backend is the whole surface fleetgauge needs from the host. Implementations
// must be safe for concurrent use: the poller, the SSE hub and the restart
// handler all call through the same value.
type Backend interface {
	// List expands the configured patterns (exact unit names and globs) into
	// the concrete unit names to watch, sorted and deduplicated.
	List(ctx context.Context, patterns []string) ([]string, error)

	// Show returns one snapshot per requested name, in the same order as the
	// input. Units that do not exist come back with Found=false, not as an
	// error: one missing unit must not blind the whole page.
	Show(ctx context.Context, names []string) ([]UnitSnapshot, error)

	// JournalTail returns up to n recent journal lines for the unit, oldest
	// first.
	JournalTail(ctx context.Context, name string, n int) ([]string, error)

	// Restart restarts the unit. Callers are responsible for the three gates
	// (bearer token, per-unit allow_restart opt-in, ledger append) before
	// reaching this method; the backend itself enforces none of them.
	Restart(ctx context.Context, name string) error
}
