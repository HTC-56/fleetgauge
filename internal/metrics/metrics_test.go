package metrics

import (
	"strings"
	"testing"
	"time"

	"fleetgauge/internal/backend"
	"fleetgauge/internal/poller"
)

// testNow returns a fixed base time so samples are ordered and reproducible.
// The poller package's pinnedClock serves the same purpose there — do not
// redefine it in the poller package.
func testNow() time.Time {
	return time.Date(2025, 6, 15, 3, 0, 0, 0, time.UTC)
}

// TestRenderHelpTypeLines asserts that every metric family appears with
// exactly one # HELP and one # TYPE, and that restarts_total is a counter
// while the rest are gauges.
func TestRenderHelpTypeLines(t *testing.T) {
	st := poller.NewStore(10)

	// Record two polls for one unit so the store is non-trivial.
	now := testNow()
	st.Record(now, []backend.UnitSnapshot{
		{Name: "app.service", Found: true, ActiveState: backend.StateActive, SubState: "running", MemoryBytes: 1 << 20},
	})
	st.Record(now, []backend.UnitSnapshot{
		{Name: "app.service", Found: true, ActiveState: backend.StateActive, SubState: "running", MemoryBytes: 1 << 20},
	})

	out := Render(st, now)

	families := []struct {
		name   string
		isType string
	}{
		{"fleetgauge_unit_up", "gauge"},
		{"fleetgauge_unit_state", "gauge"},
		{"fleetgauge_unit_restarts_total", "counter"},
		{"fleetgauge_unit_memory_bytes", "gauge"},
		{"fleetgauge_snapshot_age_seconds", "gauge"},
	}

	for _, f := range families {
		helpCount := strings.Count(out, "# HELP "+f.name+" ")
		typeCount := strings.Count(out, "# TYPE "+f.name+" "+f.isType+"\n")
		if helpCount != 1 {
			t.Errorf("# HELP %s appears %d times, want 1", f.name, helpCount)
		}
		if typeCount != 1 {
			t.Errorf("# TYPE %s %s appears %d times, want 1", f.name, f.isType, typeCount)
		}
	}
}

// TestRenderUpValues asserts that an active unit renders up 1 while a
// failed unit renders up 0.
func TestRenderUpValues(t *testing.T) {
	st := poller.NewStore(10)
	now := testNow()

	st.Record(now, []backend.UnitSnapshot{
		{Name: "nginx.service", Found: true, ActiveState: backend.StateActive, SubState: "running", MemoryBytes: 48 << 20},
		{Name: "wedged.service", Found: true, ActiveState: backend.StateFailed, SubState: "failed", MemoryBytes: 8 << 20},
	})

	out := Render(st, now)

	wantUp := "fleetgauge_unit_up{unit=\"nginx.service\"} 1\n"
	if !strings.Contains(out, wantUp) {
		t.Errorf("output missing %q", wantUp)
	}

	wantDown := "fleetgauge_unit_up{unit=\"wedged.service\"} 0\n"
	if !strings.Contains(out, wantDown) {
		t.Errorf("output missing %q", wantDown)
	}
}

// TestRenderMemoryUnknownSkipped asserts that a unit with MemoryUnknown
// produces no fleetgauge_unit_memory_bytes line, and that no memory line
// carries a negative value.
func TestRenderMemoryUnknownSkipped(t *testing.T) {
	st := poller.NewStore(10)
	now := testNow()

	st.Record(now, []backend.UnitSnapshot{
		{Name: "monitor.service", Found: true, ActiveState: backend.StateActive, SubState: "running", MemoryBytes: backend.MemoryUnknown},
		{Name: "nginx.service", Found: true, ActiveState: backend.StateActive, SubState: "running", MemoryBytes: 48 << 20},
	})

	out := Render(st, now)

	lines := strings.Split(out, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "fleetgauge_unit_memory_bytes") && strings.Contains(line, "monitor.service") {
			t.Errorf("found memory line for MemoryUnknown unit: %q", line)
		}
		if strings.HasPrefix(line, "fleetgauge_unit_memory_bytes") {
			idx := strings.Index(line, " ")
			if idx >= 0 {
				val := line[idx+1:]
				if strings.HasPrefix(val, "-") {
					t.Errorf("negative memory value found: %q", line)
				}
			}
		}
	}
}

// TestRenderSnapshotAgeOnce asserts that fleetgauge_snapshot_age_seconds
// appears exactly once and its value parses as a non-negative number.
func TestRenderSnapshotAgeOnce(t *testing.T) {
	st := poller.NewStore(10)
	now := testNow()

	st.Record(now, []backend.UnitSnapshot{
		{Name: "app.service", Found: true, ActiveState: backend.StateActive, MemoryBytes: 1 << 20},
	})

	out := Render(st, now)

	// Count only data lines (HELP/TYPE also contain the name with a trailing space).
	count := 0
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "fleetgauge_snapshot_age_seconds ") && !strings.HasPrefix(line, "# ") {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("snapshot_age appears %d times, want 1", count)
	}

	// Find the value after the space.
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "fleetgauge_snapshot_age_seconds ") {
			val := strings.TrimSpace(strings.TrimPrefix(line, "fleetgauge_snapshot_age_seconds "))
			if val == "" {
				t.Fatal("snapshot_age_seconds has no value")
			}
			// Just check it starts with a digit (non-negative).
			if val[0] < '0' || val[0] > '9' {
				t.Errorf("snapshot_age_seconds value %q is not non-negative", val)
			}
			break
		}
	}
}

// TestRenderDeterministic asserts that rendering the same store twice
// returns byte-identical output.
func TestRenderDeterministic(t *testing.T) {
	st := poller.NewStore(10)
	now := testNow()

	st.Record(now, []backend.UnitSnapshot{
		{Name: "zebra.service", Found: true, ActiveState: backend.StateActive, MemoryBytes: 100},
		{Name: "alpha.service", Found: true, ActiveState: backend.StateFailed, MemoryBytes: 200},
		{Name: "mid.service", Found: true, ActiveState: backend.StateActive, MemoryBytes: backend.MemoryUnknown},
	})

	out1 := Render(st, now)
	out2 := Render(st, now)

	if out1 != out2 {
		t.Errorf("second render differs from first\n--- first ---\n%s\n--- second ---\n%s", out1, out2)
	}
}

// TestRenderLabelEscaping asserts that a unit name containing a double quote
// is escaped as \" inside the Prometheus label value.
func TestRenderLabelEscaping(t *testing.T) {
	st := poller.NewStore(10)
	now := testNow()

	st.Record(now, []backend.UnitSnapshot{
		{Name: `app"service`, Found: true, ActiveState: backend.StateActive, MemoryBytes: 1 << 20},
	})

	out := Render(st, now)

	// The escaped label should appear in up and state lines.
	wantEscaped := `fleetgauge_unit_up{unit="app\"service"} 1`
	if !strings.Contains(out, wantEscaped) {
		t.Errorf("output missing escaped label %q\n---\n%s", wantEscaped, out)
	}
}
