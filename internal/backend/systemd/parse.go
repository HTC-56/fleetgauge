package systemd

import (
	"strconv"
	"strings"
	"time"

	"fleetgauge/internal/backend"
)

// timestampLayout matches systemd's human-readable timestamp rendering, e.g.
// "Sun 2026-08-09 17:07:01 CDT". The zone is an abbreviation, which Go can only
// resolve against a location, so parsing happens in the supplied location --
// fleetgauge runs on the same host as the systemd it is reading, so that is
// the machine's own zone.
const timestampLayout = "Mon 2006-01-02 15:04:05 MST"

// memoryNotSet is what systemd prints for MemoryCurrent when accounting is off.
const memoryNotSet = "[not set]"

// memorySentinel is uint64 max, which some systemd versions emit instead of
// "[not set]" for an unavailable MemoryCurrent.
const memorySentinel = "18446744073709551615"

// parseShow turns the output of `systemctl show <unit> -p ...` into a snapshot.
//
// The output is one KEY=VALUE per line, in unspecified order, and a value may
// legitimately be empty or contain '='. Properties absent from the output leave
// their field at the zero value, except MemoryBytes, which defaults to
// backend.MemoryUnknown so that "not measured" is never confused with "0 bytes".
//
// A unit that does not exist still produces output (LoadState=not-found), so
// existence is decided here rather than by the caller.
func parseShow(out string, loc *time.Location) backend.UnitSnapshot {
	snap := backend.UnitSnapshot{
		ActiveState: backend.StateUnknown,
		MemoryBytes: backend.MemoryUnknown,
	}

	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimRight(line, "\r")
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch key {
		case "Id":
			snap.Name = value
		case "ActiveState":
			if value != "" {
				snap.ActiveState = backend.ActiveState(value)
			}
		case "SubState":
			snap.SubState = value
		case "LoadState":
			snap.LoadState = value
		case "NRestarts":
			if n, err := strconv.Atoi(value); err == nil && n >= 0 {
				snap.NRestarts = n
			}
		case "MemoryCurrent":
			snap.MemoryBytes = parseMemory(value)
		case "ExecMainStartTimestamp":
			snap.StartedAt = parseTimestamp(value, loc)
		}
	}

	// LoadState is the reliable existence signal: a missing unit reports
	// "not-found", a masked one "masked". Anything else means systemd knows it.
	switch snap.LoadState {
	case "", "not-found":
		snap.Found = false
	default:
		snap.Found = true
	}

	return snap
}

// parseMemory reads MemoryCurrent, mapping every "unavailable" spelling to
// backend.MemoryUnknown.
func parseMemory(value string) int64 {
	value = strings.TrimSpace(value)
	if value == "" || value == memoryNotSet || value == memorySentinel {
		return backend.MemoryUnknown
	}
	n, err := strconv.ParseInt(value, 10, 64)
	if err != nil || n < 0 {
		return backend.MemoryUnknown
	}
	return n
}

// parseTimestamp reads ExecMainStartTimestamp. An empty value (the unit has
// never run) and an unparseable one both yield the zero Time; callers check
// IsZero before computing uptime.
func parseTimestamp(value string, loc *time.Location) time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}
	}
	if loc == nil {
		loc = time.Local
	}
	t, err := time.ParseInLocation(timestampLayout, value, loc)
	if err != nil {
		return time.Time{}
	}
	return t
}

// splitShowRecords splits the output of a multi-unit `systemctl show` into one
// chunk per unit. systemd separates records with a blank line when several
// units are queried in one call.
func splitShowRecords(out string) []string {
	blocks := strings.Split(strings.ReplaceAll(out, "\r\n", "\n"), "\n\n")
	records := make([]string, 0, len(blocks))
	for _, b := range blocks {
		if strings.TrimSpace(b) != "" {
			records = append(records, b)
		}
	}
	return records
}
