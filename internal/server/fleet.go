package server

import (
	"encoding/json"
	"log/slog"
	"os"
	"sync"
	"time"

	"fleetgauge/internal/backend"
	"fleetgauge/internal/ledger"
	"fleetgauge/internal/poller"
)

// Appender is the slice of internal/ledger the restart handler needs.
//
// It is an interface rather than a *ledger.Ledger so that a test can inject a
// writer that fails, and prove the handler refuses the restart when the action
// cannot be recorded.
type Appender interface {
	Append(ledger.Entry) error
}

// Defaults applied by New when an Options field is left zero.
const (
	DefaultBroadcastInterval = 2 * time.Second
	DefaultHeartbeat         = 20 * time.Second
	DefaultJournalLines      = 50

	// MaxTransitionsShown caps how many transitions ride along in a fleet
	// payload. The timeline is a recent-history strip, not an audit log; the
	// store keeps more than the page needs to draw.
	MaxTransitionsShown = 50
)

// UnitJSON is one row of the fleet as the page sees it.
//
// Field names are snake_case because they are consumed by hand-written
// JavaScript in internal/page/index.html, not by another Go program. Changing
// one here means changing it there.
type UnitJSON struct {
	Name        string  `json:"name"`
	Found       bool    `json:"found"`
	ActiveState string  `json:"active_state"`
	SubState    string  `json:"sub_state"`
	Restarts    int     `json:"restarts"`
	MemoryBytes int64   `json:"memory_bytes"` // backend.MemoryUnknown (-1) when accounting is off
	UptimeSec   float64 `json:"uptime_seconds"`
	ObservedAt  string  `json:"observed_at"` // RFC3339, empty when never observed
	Samples     int     `json:"samples"`
	Transitions int     `json:"transitions"`

	// AllowRestart mirrors the per-unit opt-in from the config. The restart
	// verb itself is not built yet; the page uses this only to decide whether
	// a unit could ever be restarted.
	AllowRestart bool `json:"allow_restart"`
}

// TransitionJSON is one observed state change, as the page's timeline sees it.
type TransitionJSON struct {
	Unit string `json:"unit"`
	From string `json:"from"`
	To   string `json:"to"`
	At   string `json:"at"` // RFC3339
}

// CountsJSON is the fleet-wide tally the page's header strip shows.
type CountsJSON struct {
	Total  int `json:"total"`
	Active int `json:"active"`
	Failed int `json:"failed"`
	Other  int `json:"other"`
}

// FleetJSON is the whole payload: one snapshot of everything the page draws.
// It is what /events pushes as the "fleet" event.
type FleetJSON struct {
	Units          []UnitJSON       `json:"units"`
	Transitions    []TransitionJSON `json:"transitions"` // newest first
	Counts         CountsJSON       `json:"counts"`
	SnapshotAgeSec float64          `json:"snapshot_age_seconds"`
	Polls          int              `json:"polls"`
	Failures       int              `json:"failures"`
	LastError      string           `json:"last_error,omitempty"`
	GeneratedAt    string           `json:"generated_at"`
}

// Options configures a Server. Only Store is required.
type Options struct {
	// Store is the history every handler reads. Required.
	Store *poller.Store

	// Backend is used only by the journal drawer, which fetches lines on
	// demand. A nil Backend makes the journal endpoint report that journals
	// are unavailable rather than crashing.
	Backend backend.Backend

	// AllowRestart is the per-unit restart opt-in from the config, keyed by
	// unit name. Absent means false.
	AllowRestart map[string]bool

	// BearerToken authenticates the restart verb. An empty token disables
	// restart entirely — an unset token must never be read as "anyone may
	// restart anything".
	BearerToken string

	// Ledger records restart attempts. A nil Ledger disables the restart verb,
	// because SPEC.md requires the append to happen before the backend is
	// touched: an action that cannot be recorded is not performed.
	Ledger Appender

	// JournalLines caps how many lines the journal drawer returns.
	JournalLines int

	// BroadcastInterval is how often Broadcast pushes a fleet event.
	BroadcastInterval time.Duration

	// Heartbeat is how often an idle SSE connection gets a comment line, so
	// proxies and load balancers do not reap it as dead.
	Heartbeat time.Duration

	// Logger receives structured logs. Nil means a text handler on stderr.
	Logger *slog.Logger

	// Now is the clock, injected so tests can pin timestamps. Nil means
	// time.Now.
	Now func() time.Time
}

// Server holds everything the HTTP handlers need. It is safe for concurrent
// use; the mutex guards only the broadcast watermark.
type Server struct {
	store        *poller.Store
	be           backend.Backend
	allowRestart map[string]bool
	bearerToken  string
	ledger       Appender
	journalLines int
	interval     time.Duration
	heartbeat    time.Duration
	log          *slog.Logger
	hub          *Hub
	now          func() time.Time

	mu           sync.Mutex
	lastTr       poller.Transition
	watermarkSet bool
}

// New returns a Server reading opts.Store. Zero-valued options fall back to
// the package defaults rather than to zero timers, which would spin.
func New(opts Options) *Server {
	if opts.BroadcastInterval <= 0 {
		opts.BroadcastInterval = DefaultBroadcastInterval
	}
	if opts.Heartbeat <= 0 {
		opts.Heartbeat = DefaultHeartbeat
	}
	if opts.JournalLines <= 0 {
		opts.JournalLines = DefaultJournalLines
	}
	if opts.Logger == nil {
		opts.Logger = slog.New(slog.NewTextHandler(os.Stderr, nil))
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.AllowRestart == nil {
		opts.AllowRestart = map[string]bool{}
	}

	return &Server{
		store:        opts.Store,
		be:           opts.Backend,
		allowRestart: opts.AllowRestart,
		bearerToken:  opts.BearerToken,
		ledger:       opts.Ledger,
		journalLines: opts.JournalLines,
		interval:     opts.BroadcastInterval,
		heartbeat:    opts.Heartbeat,
		log:          opts.Logger,
		hub:          NewHub(),
		now:          opts.Now,
	}
}

// Store returns the history this server reads.
func (s *Server) Store() *poller.Store { return s.store }

// Hub returns the SSE fan-out /events subscribes to.
func (s *Server) Hub() *Hub { return s.hub }

// Log returns the structured logger.
func (s *Server) Log() *slog.Logger { return s.log }

// JournalLines reports the configured journal-drawer depth.
func (s *Server) JournalLines() int { return s.journalLines }

// Backend returns the backend the journal drawer reads, which may be nil.
func (s *Server) Backend() backend.Backend { return s.be }

// AllowRestart reports whether the named unit opted in to restart.
func (s *Server) AllowRestart(name string) bool { return s.allowRestart[name] }

// RestartEnabled reports whether the restart verb is configured at all: it
// needs a bearer token to check, a ledger to write to, and a backend to act
// through. Missing any one of the three makes the endpoint report 503 rather
// than pretend to be a working control.
func (s *Server) RestartEnabled() bool {
	return s.bearerToken != "" && s.ledger != nil && s.be != nil
}

// Now returns the server's clock reading.
func (s *Server) Now() time.Time { return s.now() }

// Close releases the SSE hub, ending every open stream.
func (s *Server) Close() { s.hub.Close() }

// Fleet builds the payload the page draws: one row per unit, the recent
// transition timeline newest first, and the poll bookkeeping.
func (s *Server) Fleet() FleetJSON {
	now := s.now()

	views := s.store.Overview(now)
	units := make([]UnitJSON, 0, len(views))
	var counts CountsJSON
	for _, v := range views {
		observed := ""
		if !v.ObservedAt.IsZero() {
			observed = v.ObservedAt.UTC().Format(time.RFC3339)
		}
		units = append(units, UnitJSON{
			Name:         v.Name,
			Found:        v.Found,
			ActiveState:  string(v.ActiveState),
			SubState:     v.SubState,
			Restarts:     v.NRestarts,
			MemoryBytes:  v.MemoryBytes,
			UptimeSec:    v.Uptime.Seconds(),
			ObservedAt:   observed,
			Samples:      v.Samples,
			Transitions:  v.Transitions,
			AllowRestart: s.allowRestart[v.Name],
		})

		counts.Total++
		switch v.ActiveState {
		case backend.StateActive:
			counts.Active++
		case backend.StateFailed:
			counts.Failed++
		default:
			counts.Other++
		}
	}

	all := s.store.Transitions()
	if len(all) > MaxTransitionsShown {
		all = all[len(all)-MaxTransitionsShown:]
	}
	// Reverse into newest-first: the timeline reads top-down as "what just
	// happened", which is the question the page exists to answer.
	trs := make([]TransitionJSON, 0, len(all))
	for i := len(all) - 1; i >= 0; i-- {
		t := all[i]
		trs = append(trs, TransitionJSON{
			Unit: t.Unit,
			From: string(t.From),
			To:   string(t.To),
			At:   t.At.UTC().Format(time.RFC3339),
		})
	}

	polls, failures := s.store.Counts()
	_, lastErr := s.store.LastPoll()
	errText := ""
	if lastErr != nil {
		errText = lastErr.Error()
	}

	return FleetJSON{
		Units:          units,
		Transitions:    trs,
		Counts:         counts,
		SnapshotAgeSec: s.store.SnapshotAge(now).Seconds(),
		Polls:          polls,
		Failures:       failures,
		LastError:      errText,
		GeneratedAt:    now.UTC().Format(time.RFC3339),
	}
}

// mustJSON encodes v, returning "{}" if encoding somehow fails. A monitoring
// daemon does not panic because one payload would not marshal.
func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}
