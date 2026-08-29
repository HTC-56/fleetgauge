// Package metrics renders the poller store as Prometheus exposition-format
// text. It is a pure function of the store's overview — no HTTP, no side
// effects — so the tests can assert on it without systemd or a running daemon.
package metrics

import (
	"strconv"
	"strings"
	"time"

	"fleetgauge/internal/backend"
	"fleetgauge/internal/poller"
)

// Render returns the full Prometheus exposition text for the store at now.
//
// Five families are emitted, in order: unit_up, unit_state,
// unit_restarts_total, unit_memory_bytes, snapshot_age_seconds. Each family
// is preceded by one HELP and one TYPE line. Within a family one line per
// unit appears in Overview order (sorted by unit name).
//
// fleetgauge_unit_up is 1 when the unit's ActiveState equals
// backend.StateActive, else 0. fleetgauge_unit_state emits one labelled line
// per unit for its current state. fleetgauge_unit_restarts_total carries the
// restart counter. fleetgauge_unit_memory_bytes is omitted entirely for units
// whose MemoryBytes is backend.MemoryUnknown.
//
// fleetgauge_snapshot_age_seconds is a single unlabelled line showing how
// long ago the last successful poll happened.
func Render(st *poller.Store, now time.Time) string {
	views := st.Overview(now)

	var sb strings.Builder

	// --- fleetgauge_unit_up ---
	sb.WriteString("# HELP fleetgauge_unit_up Whether a unit is active.\n")
	sb.WriteString("# TYPE fleetgauge_unit_up gauge\n")
	for _, v := range views {
		sb.WriteString("fleetgauge_unit_up{unit=\"")
		sb.WriteString(escape(v.Name))
		sb.WriteString("\"} ")
		if v.ActiveState == backend.StateActive {
			sb.WriteString("1\n")
		} else {
			sb.WriteString("0\n")
		}
	}

	// --- fleetgauge_unit_state ---
	sb.WriteString("# HELP fleetgauge_unit_state Current state of a unit (always 1).\n")
	sb.WriteString("# TYPE fleetgauge_unit_state gauge\n")
	for _, v := range views {
		sb.WriteString("fleetgauge_unit_state{unit=\"")
		sb.WriteString(escape(v.Name))
		sb.WriteString("\",state=\"")
		sb.WriteString(escape(string(v.ActiveState)))
		sb.WriteString("\"} 1\n")
	}

	// --- fleetgauge_unit_restarts_total ---
	sb.WriteString("# HELP fleetgauge_unit_restarts_total Total restarts for a unit.\n")
	sb.WriteString("# TYPE fleetgauge_unit_restarts_total counter\n")
	for _, v := range views {
		sb.WriteString("fleetgauge_unit_restarts_total{unit=\"")
		sb.WriteString(escape(v.Name))
		sb.WriteString("\"} ")
		sb.WriteString(strconv.Itoa(v.NRestarts))
		sb.WriteString("\n")
	}

	// --- fleetgauge_unit_memory_bytes ---
	sb.WriteString("# HELP fleetgauge_unit_memory_bytes MemoryCurrent for a unit.\n")
	sb.WriteString("# TYPE fleetgauge_unit_memory_bytes gauge\n")
	for _, v := range views {
		if v.MemoryBytes == backend.MemoryUnknown {
			continue
		}
		sb.WriteString("fleetgauge_unit_memory_bytes{unit=\"")
		sb.WriteString(escape(v.Name))
		sb.WriteString("\"} ")
		sb.WriteString(strconv.FormatInt(v.MemoryBytes, 10))
		sb.WriteString("\n")
	}

	// --- fleetgauge_snapshot_age_seconds ---
	sb.WriteString("# HELP fleetgauge_snapshot_age_seconds Seconds since the last successful poll.\n")
	sb.WriteString("# TYPE fleetgauge_snapshot_age_seconds gauge\n")
	sb.WriteString("fleetgauge_snapshot_age_seconds ")
	sb.WriteString(strconv.FormatFloat(st.SnapshotAge(now).Seconds(), 'f', -1, 64))
	sb.WriteString("\n")

	return sb.String()
}

// escape returns val with backslash, double-quote and newline escaped for a
// Prometheus label value: \ -> \\, " -> \", newline -> \n.
func escape(val string) string {
	var b strings.Builder
	for _, r := range val {
		switch r {
		case '\\':
			b.WriteString("\\\\")
		case '"':
			b.WriteString("\\\"")
		case '\n':
			b.WriteString("\\n")
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
