// Package systemd is the real backend: it reads unit state by running
// `systemctl show` and `journalctl` as subprocesses and parsing their output.
// No cgo, no D-Bus, no library — just the two binaries every systemd box has.
//
// This package is the ONLY place in the repo allowed to call exec.Command.
// scripts/scrub-check.sh enforces that; the seam is the point of the design.
package systemd

import (
	"context"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"time"

	"fleetgauge/internal/backend"
)

// showProperties are exactly the properties fleetgauge parses. Asking for a
// fixed set keeps the output small and the parser honest.
var showProperties = []string{
	"Id",
	"LoadState",
	"ActiveState",
	"SubState",
	"NRestarts",
	"MemoryCurrent",
	"ExecMainStartTimestamp",
}

// Backend reads real units through systemctl/journalctl.
//
// The zero value is usable: it shells out to "systemctl" and "journalctl" on
// PATH and parses timestamps in the local zone. It holds no mutable state, so
// it is trivially safe for concurrent use.
type Backend struct {
	// SystemctlPath overrides the systemctl binary. Empty means "systemctl".
	SystemctlPath string
	// JournalctlPath overrides the journalctl binary. Empty means "journalctl".
	JournalctlPath string
	// Location parses systemd timestamps. Nil means time.Local.
	Location *time.Location
}

// New returns a Backend with the defaults described on the struct.
func New() *Backend { return &Backend{} }

var _ backend.Backend = (*Backend)(nil)

func (b *Backend) systemctl() string {
	if b.SystemctlPath != "" {
		return b.SystemctlPath
	}
	return "systemctl"
}

func (b *Backend) journalctl() string {
	if b.JournalctlPath != "" {
		return b.JournalctlPath
	}
	return "journalctl"
}

func (b *Backend) location() *time.Location {
	if b.Location != nil {
		return b.Location
	}
	return time.Local
}

// run executes a command and returns its stdout. Stderr is folded into the
// error so a failure reports why rather than just an exit status.
func run(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return "", fmt.Errorf("%s: %w: %s", name, err, msg)
		}
		return "", fmt.Errorf("%s: %w", name, err)
	}
	return string(out), nil
}

// List expands patterns into concrete unit names via `systemctl list-units`.
// Exact names (no glob metacharacters) are kept even when systemd does not
// list them, so a unit that is configured but missing still reaches the page
// as Found=false instead of vanishing.
func (b *Backend) List(ctx context.Context, patterns []string) ([]string, error) {
	seen := make(map[string]struct{})

	for _, p := range patterns {
		if p == "" {
			continue
		}
		if !strings.ContainsAny(p, "*?[") {
			seen[p] = struct{}{}
			continue
		}
		args := []string{"list-units", "--all", "--no-legend", "--plain",
			"--no-pager", "--type=service", p}
		out, err := run(ctx, b.systemctl(), args...)
		if err != nil {
			return nil, fmt.Errorf("list units for %q: %w", p, err)
		}
		for _, line := range strings.Split(out, "\n") {
			fields := strings.Fields(line)
			if len(fields) == 0 {
				continue
			}
			seen[fields[0]] = struct{}{}
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
func (b *Backend) Show(ctx context.Context, names []string) ([]backend.UnitSnapshot, error) {
	if len(names) == 0 {
		return nil, nil
	}

	args := []string{"show", "--no-pager"}
	for _, p := range showProperties {
		args = append(args, "-p", p)
	}
	args = append(args, names...)

	out, err := run(ctx, b.systemctl(), args...)
	if err != nil {
		return nil, fmt.Errorf("show units: %w", err)
	}

	records := splitShowRecords(out)
	loc := b.location()

	// Index parsed records by Id so the result can be returned in request
	// order regardless of how systemd ordered its output.
	byName := make(map[string]backend.UnitSnapshot, len(records))
	ordered := make([]backend.UnitSnapshot, 0, len(records))
	for _, rec := range records {
		snap := parseShow(rec, loc)
		if snap.Name != "" {
			byName[snap.Name] = snap
		}
		ordered = append(ordered, snap)
	}

	snaps := make([]backend.UnitSnapshot, len(names))
	for i, name := range names {
		if snap, ok := byName[name]; ok {
			snaps[i] = snap
			continue
		}
		// Fall back to positional matching for the single-unit case, where a
		// nonexistent unit may come back without an Id line.
		if len(names) == len(ordered) && ordered[i].Name == "" {
			snap := ordered[i]
			snap.Name = name
			snaps[i] = snap
			continue
		}
		snaps[i] = backend.UnitSnapshot{
			Name:        name,
			Found:       false,
			ActiveState: backend.StateUnknown,
			MemoryBytes: backend.MemoryUnknown,
		}
	}
	return snaps, nil
}

// JournalTail returns up to n recent journal lines for the unit, oldest first.
func (b *Backend) JournalTail(ctx context.Context, name string, n int) ([]string, error) {
	if n <= 0 {
		return nil, nil
	}
	args := []string{"-u", name, "-n", fmt.Sprint(n), "--no-pager", "--output=short-iso"}
	out, err := run(ctx, b.journalctl(), args...)
	if err != nil {
		return nil, fmt.Errorf("journal tail for %q: %w", name, err)
	}
	lines := make([]string, 0, n)
	for _, line := range strings.Split(strings.ReplaceAll(out, "\r\n", "\n"), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		lines = append(lines, line)
	}
	return lines, nil
}

// Restart restarts the unit. The three gates live above this method.
func (b *Backend) Restart(ctx context.Context, name string) error {
	if _, err := run(ctx, b.systemctl(), "restart", name); err != nil {
		return fmt.Errorf("restart %q: %w", name, err)
	}
	return nil
}
