// Package ledger appends fleetgauge's action records to a JSONL file.
//
// Restart is the only mutating verb in the product, and SPEC.md requires that
// every attempt be written down BEFORE it is executed. That ordering is the
// whole point: a process killed mid-restart must leave evidence that the
// restart was authorised and started, not silence. So a restart writes two
// lines — one when it is authorised, one when it finishes — and an
// interrupted restart is visible as a requested line with no outcome.
//
// One JSON object per line, append-only, never rewritten. The file is opened
// O_APPEND, so a second fleetgauge pointed at the same path interleaves whole
// lines instead of clobbering them.
package ledger

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"
)

// Action values. Restart is the only one v1 records; the constant exists so
// that a reader can filter on a stable string rather than on a literal.
const ActionRestart = "restart"

// Result values. Every authorised restart writes ResultRequested before the
// backend is touched, then exactly one of ResultOK or ResultError after it
// returns.
const (
	ResultRequested = "requested"
	ResultOK        = "ok"
	ResultError     = "error"
)

// ErrClosed is returned by Append on a ledger that has been closed, or on a
// nil *Ledger. A caller that cannot record an action must refuse to perform
// it, so this is an error rather than a silent no-op.
var ErrClosed = errors.New("ledger: closed")

// Entry is one line of the ledger.
//
// Field names are snake_case to match the rest of fleetgauge's JSON surface.
// The file is meant to be read with jq or grep, so every field is a scalar.
type Entry struct {
	At     time.Time `json:"at"`
	Action string    `json:"action"`
	Unit   string    `json:"unit"`
	Actor  string    `json:"actor,omitempty"` // request source address
	Result string    `json:"result"`
	Error  string    `json:"error,omitempty"`
}

// Ledger is an append-only JSONL writer, safe for concurrent use.
type Ledger struct {
	mu   sync.Mutex
	f    *os.File
	path string
}

// Open opens the ledger at path, creating it if it does not exist and
// appending to it if it does. An existing ledger is never truncated: the
// action history outlives the process that wrote it.
//
// Mode 0o640 keeps the file readable by the service group and nobody else;
// it records who restarted what and when.
func Open(path string) (*Ledger, error) {
	if path == "" {
		return nil, errors.New("ledger: path must not be empty")
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o640)
	if err != nil {
		return nil, fmt.Errorf("ledger: open %s: %w", path, err)
	}
	return &Ledger{f: f, path: path}, nil
}

// Path reports the file this ledger writes to.
func (l *Ledger) Path() string {
	if l == nil {
		return ""
	}
	return l.path
}

// Append writes e as one JSON line and syncs it to disk before returning.
//
// The fsync is deliberate. Append is called once per restart request, so the
// cost is a syscall on a human-initiated action, and it is what makes "the
// record exists before the restart runs" true across a power loss rather than
// only across a clean exit.
func (l *Ledger) Append(e Entry) error {
	if l == nil {
		return ErrClosed
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.f == nil {
		return ErrClosed
	}

	b, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("ledger: marshal: %w", err)
	}
	b = append(b, '\n')

	// One Write of one complete line: O_APPEND makes a single write atomic
	// against other appenders, so lines never interleave mid-record.
	if _, err := l.f.Write(b); err != nil {
		return fmt.Errorf("ledger: write %s: %w", l.path, err)
	}
	if err := l.f.Sync(); err != nil {
		return fmt.Errorf("ledger: sync %s: %w", l.path, err)
	}
	return nil
}

// Close closes the underlying file. It is safe to call more than once; the
// second call is a no-op. Append after Close returns ErrClosed.
func (l *Ledger) Close() error {
	if l == nil {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.f == nil {
		return nil
	}
	f := l.f
	l.f = nil
	return f.Close()
}
