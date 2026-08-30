package systemd

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/HTC-56/fleetgauge/internal/backend"
)

func readFixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return string(b)
}

// The round-trip the spec pre-registers: real `systemctl show` output in,
// a fully populated snapshot out.
func TestParseShowActiveUnit(t *testing.T) {
	snap := parseShow(readFixture(t, "show_active.txt"), time.UTC)

	if snap.Name != "nginx.service" {
		t.Errorf("Name = %q, want nginx.service", snap.Name)
	}
	if !snap.Found {
		t.Error("Found = false, want true for a loaded unit")
	}
	if snap.ActiveState != backend.StateActive {
		t.Errorf("ActiveState = %q, want active", snap.ActiveState)
	}
	if snap.SubState != "running" {
		t.Errorf("SubState = %q, want running", snap.SubState)
	}
	if snap.NRestarts != 3 {
		t.Errorf("NRestarts = %d, want 3", snap.NRestarts)
	}
	if snap.MemoryBytes != 1146880 {
		t.Errorf("MemoryBytes = %d, want 1146880", snap.MemoryBytes)
	}
	want := time.Date(2026, 8, 9, 17, 7, 1, 0, time.UTC)
	if !snap.StartedAt.Equal(want) {
		t.Errorf("StartedAt = %v, want %v", snap.StartedAt, want)
	}
}

// A unit named in config but absent on the box must come back Found=false with
// memory reported as unknown -- never as zero bytes.
func TestParseShowMissingUnit(t *testing.T) {
	snap := parseShow(readFixture(t, "show_missing.txt"), time.UTC)

	if snap.Found {
		t.Error("Found = true, want false for LoadState=not-found")
	}
	if snap.MemoryBytes != backend.MemoryUnknown {
		t.Errorf("MemoryBytes = %d, want MemoryUnknown (%d)", snap.MemoryBytes, backend.MemoryUnknown)
	}
	if !snap.StartedAt.IsZero() {
		t.Errorf("StartedAt = %v, want zero for a never-started unit", snap.StartedAt)
	}
	if snap.Uptime(time.Now()) != 0 {
		t.Error("Uptime should be 0 when the unit never started")
	}
}

func TestParseMemory(t *testing.T) {
	cases := []struct {
		in   string
		want int64
	}{
		{"1146880", 1146880},
		{"0", 0},
		{"[not set]", backend.MemoryUnknown},
		{"", backend.MemoryUnknown},
		{"18446744073709551615", backend.MemoryUnknown}, // uint64 max sentinel
		{"garbage", backend.MemoryUnknown},
		{"-5", backend.MemoryUnknown},
	}
	for _, c := range cases {
		if got := parseMemory(c.in); got != c.want {
			t.Errorf("parseMemory(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

// "0 bytes measured" and "accounting is off" must stay distinguishable; that
// distinction is why MemoryUnknown is -1 rather than 0.
func TestMemoryZeroIsNotUnknown(t *testing.T) {
	if parseMemory("0") == backend.MemoryUnknown {
		t.Error("MemoryCurrent=0 must not collapse into MemoryUnknown")
	}
}

func TestParseTimestampEmptyAndGarbage(t *testing.T) {
	if got := parseTimestamp("", time.UTC); !got.IsZero() {
		t.Errorf("empty timestamp = %v, want zero", got)
	}
	if got := parseTimestamp("not a timestamp", time.UTC); !got.IsZero() {
		t.Errorf("garbage timestamp = %v, want zero", got)
	}
}

// Values may be empty or contain '='; a line without '=' is skipped entirely.
func TestParseShowOddLines(t *testing.T) {
	out := "Id=odd.service\nLoadState=loaded\nSubState=\nActiveState=failed\nnot-a-property\nEnvironment=A=B\n"
	snap := parseShow(out, time.UTC)

	if snap.ActiveState != backend.StateFailed {
		t.Errorf("ActiveState = %q, want failed", snap.ActiveState)
	}
	if snap.SubState != "" {
		t.Errorf("SubState = %q, want empty", snap.SubState)
	}
	if !snap.Found {
		t.Error("Found = false, want true for LoadState=loaded")
	}
}

// Absent properties must not silently read as zero-valued measurements.
func TestParseShowEmptyInput(t *testing.T) {
	snap := parseShow("", time.UTC)

	if snap.Found {
		t.Error("Found = true for empty output, want false")
	}
	if snap.ActiveState != backend.StateUnknown {
		t.Errorf("ActiveState = %q, want unknown", snap.ActiveState)
	}
	if snap.MemoryBytes != backend.MemoryUnknown {
		t.Errorf("MemoryBytes = %d, want MemoryUnknown", snap.MemoryBytes)
	}
}

func TestSplitShowRecords(t *testing.T) {
	out := "Id=a.service\nActiveState=active\n\nId=b.service\nActiveState=failed\n"
	recs := splitShowRecords(out)
	if len(recs) != 2 {
		t.Fatalf("got %d records, want 2", len(recs))
	}
	if got := parseShow(recs[0], time.UTC).Name; got != "a.service" {
		t.Errorf("first record Name = %q, want a.service", got)
	}
	if got := parseShow(recs[1], time.UTC).Name; got != "b.service" {
		t.Errorf("second record Name = %q, want b.service", got)
	}
}

func TestUptimeOnlyWhenActive(t *testing.T) {
	now := time.Date(2026, 8, 9, 18, 7, 1, 0, time.UTC)
	started := time.Date(2026, 8, 9, 17, 7, 1, 0, time.UTC)

	active := backend.UnitSnapshot{ActiveState: backend.StateActive, StartedAt: started}
	if got := active.Uptime(now); got != time.Hour {
		t.Errorf("Uptime = %v, want 1h", got)
	}

	dead := backend.UnitSnapshot{ActiveState: backend.StateInactive, StartedAt: started}
	if got := dead.Uptime(now); got != 0 {
		t.Errorf("Uptime = %v, want 0 for an inactive unit", got)
	}
}
