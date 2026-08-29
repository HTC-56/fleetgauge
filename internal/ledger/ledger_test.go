package ledger

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// readEntries reads the ledger file and returns every non-empty line decoded
// as an Entry.  Each line is a complete JSON object.
func readEntries(t *testing.T, path string) []Entry {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var entries []Entry
	for _, line := range splitLines(data) {
		if line == "" {
			continue
		}
		var e Entry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Fatalf("unmarshal line %q: %v", line, err)
		}
		entries = append(entries, e)
	}
	return entries
}

// splitLines is a minimal "\n" splitter — the spec says "split on '\n'".
func splitLines(data []byte) []string {
	var out []string
	start := 0
	for i, b := range data {
		if b == '\n' {
			out = append(out, string(data[start:i]))
			start = i + 1
		}
	}
	if start < len(data) {
		out = append(out, string(data[start:]))
	}
	return out
}

// TestOpenCreatesFile asserts that Open on a non-existent path creates the
// file and that Path() returns the path that was given.
func TestOpenCreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "actions.jsonl")

	if _, err := os.Stat(path); err == nil {
		t.Fatal("file should not exist yet")
	}

	l, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer l.Close()

	if l.Path() != path {
		t.Errorf("Path() = %q, want %q", l.Path(), path)
	}

	if _, err := os.Stat(path); err != nil {
		t.Errorf("file should exist after Open: %v", err)
	}
}

// TestAppendTwoLines asserts that after two Append calls the file holds exactly
// two lines, each of which is one complete JSON object — decoding line 1 alone
// succeeds.
func TestAppendTwoLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "actions.jsonl")

	l, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer l.Close()

	e1 := Entry{Action: ActionRestart, Unit: "a.service", Result: ResultRequested}
	e2 := Entry{Action: ActionRestart, Unit: "b.service", Result: ResultOK}

	if err := l.Append(e1); err != nil {
		t.Fatalf("Append 1: %v", err)
	}
	if err := l.Append(e2); err != nil {
		t.Fatalf("Append 2: %v", err)
	}

	entries := readEntries(t, path)
	if len(entries) != 2 {
		t.Fatalf("len(entries) = %d, want 2", len(entries))
	}

	// Decode line 1 alone by reading just the file and re-parsing.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	lines := splitLines(data)
	var first Entry
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
		t.Fatalf("decode line 1 alone: %v", err)
	}
	if first.Action != ActionRestart {
		t.Errorf("line 1 action = %q, want %q", first.Action, ActionRestart)
	}
}

// TestEntryRoundTrip asserts that every field survives the round trip: append an
// Entry with a non-zero At, plus Action, Unit, Actor, Result and Error all set,
// decode it back, and compare each field.
func TestEntryRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "actions.jsonl")

	l, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer l.Close()

	want := Entry{
		At:     time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC),
		Action: ActionRestart,
		Unit:   "flappy.service",
		Actor:  "192.0.2.5",
		Result: ResultRequested,
		Error:  "intentional failure",
	}

	if err := l.Append(want); err != nil {
		t.Fatalf("Append: %v", err)
	}

	entries := readEntries(t, path)
	if len(entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1", len(entries))
	}

	got := entries[0]
	if !got.At.Equal(want.At) {
		t.Errorf("At = %v, want %v", got.At, want.At)
	}
	if got.Action != want.Action {
		t.Errorf("Action = %q, want %q", got.Action, want.Action)
	}
	if got.Unit != want.Unit {
		t.Errorf("Unit = %q, want %q", got.Unit, want.Unit)
	}
	if got.Actor != want.Actor {
		t.Errorf("Actor = %q, want %q", got.Actor, want.Actor)
	}
	if got.Result != want.Result {
		t.Errorf("Result = %q, want %q", got.Result, want.Result)
	}
	if got.Error != want.Error {
		t.Errorf("Error = %q, want %q", got.Error, want.Error)
	}
}

// TestReopenAppends asserts that reopening an existing ledger appends rather
// than truncates: Open, append one, Close, Open again, append one more, and
// the file holds two lines.
func TestReopenAppends(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "actions.jsonl")

	l1, err := Open(path)
	if err != nil {
		t.Fatalf("Open 1: %v", err)
	}
	if err := l1.Append(Entry{Action: ActionRestart, Unit: "a.service", Result: ResultRequested}); err != nil {
		t.Fatalf("Append 1: %v", err)
	}
	if err := l1.Close(); err != nil {
		t.Fatalf("Close 1: %v", err)
	}

	l2, err := Open(path)
	if err != nil {
		t.Fatalf("Open 2: %v", err)
	}
	defer l2.Close()

	if err := l2.Append(Entry{Action: ActionRestart, Unit: "b.service", Result: ResultOK}); err != nil {
		t.Fatalf("Append 2: %v", err)
	}

	entries := readEntries(t, path)
	if len(entries) != 2 {
		t.Fatalf("len(entries) = %d, want 2", len(entries))
	}
}

// TestAppendAfterClose asserts that Append after Close returns ErrClosed and a
// second Close returns nil rather than panicking.
func TestAppendAfterClose(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "actions.jsonl")

	l, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	if err := l.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if err := l.Append(Entry{Action: ActionRestart, Unit: "x.service", Result: ResultRequested}); !errors.Is(err, ErrClosed) {
		t.Errorf("Append after Close = %v, want ErrClosed", err)
	}

	// Second Close should be a no-op, not panic.
	if err := l.Close(); err != nil {
		t.Errorf("second Close = %v, want nil", err)
	}
}

// TestConcurrentAppends asserts that 20 concurrent goroutines each appending
// once yield exactly 20 lines, every one of which decodes cleanly.
func TestConcurrentAppends(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "actions.jsonl")

	l, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer l.Close()

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			err := l.Append(Entry{
				At:     time.Now(),
				Action: ActionRestart,
				Unit:   "worker.service",
				Result: ResultRequested,
			})
			if err != nil {
				t.Errorf("goroutine %d Append: %v", n, err)
			}
		}(i)
	}
	wg.Wait()

	entries := readEntries(t, path)
	if len(entries) != 20 {
		t.Errorf("len(entries) = %d, want 20", len(entries))
	}
}
