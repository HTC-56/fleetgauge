# Phase B — the poller, the history, and the metrics text

**Subject: ROADMAP row 3, "Poller + ring-buffer history" (NOT BUILT).** This
phase carries row 3 to SHIPPED and row 5 (`/metrics`) to PARTIAL: the Prometheus
renderer is a pure function over the history, so it is built and tested here.
The endpoint that serves it waits for the HTTP phase.

**Already shipped by the planning lane** (commits `feat(B1)`–`feat(B3)`) — do
not rewrite these, build against them:

- `internal/poller/ring.go` — `Sample` and the unexported fixed-capacity `ring`
  (`newRing`, `add`, `all`, `last`, `length`, `capacity`).
- `internal/poller/store.go` — `Transition`, `Store`, `NewStore(depth)`,
  `Record`, `RecordError`, and the read accessors `Names`, `History`, `Latest`,
  `Transitions`, `LastPoll`, `SnapshotAge`, `Counts`, `Depth`.
- `internal/poller/poller.go` — `Poller`, `New`, `PollOnce`, `Run`, the
  injectable `Now` clock, `DefaultInterval`, `DefaultDepth`.
- `internal/poller/poller_smoke_test.go` — a thin end-to-end smoke test.
  **Your pattern file for every test task below.** It also defines a
  `pinnedClock()` helper: reuse it, never redefine it — a second definition in
  the same package is a compile error.

**The gate for every task below** (all four, every time; red = not done):

```
gofmt -l .            # must print nothing
go vet ./...
go test ./...
bash scripts/scrub-check.sh
```

---

## §B4 — Ring buffer tests

Create `internal/poller/ring_test.go`. Package `poller` (an internal test file,
so it can use the unexported `ring` directly).

Mirror the plain-`testing` style of `internal/poller/poller_smoke_test.go` — no
test framework, no new dependency. Build samples with distinct `At` times and a
`backend.UnitSnapshot` carrying at least a `Name`, so you can tell one sample
from another.

Assert, in prose:

1. `newRing(3)` reports `capacity() == 3` and `length() == 0`, and `last()`
   returns `false` for its second result.
2. After adding two samples, `length() == 2` and `all()` returns those two
   oldest first.
3. Adding five samples to a ring of capacity 3 leaves `length() == 3` and
   `capacity() == 3`, and `all()` returns the **last three** added, oldest
   first — the two oldest were overwritten.
4. `last()` returns the most recently added sample, both before and after the
   ring has wrapped.
5. `newRing(0)` and `newRing(-1)` each yield a ring of capacity 1 that accepts
   an add without panicking.
6. `all()` hands back a fresh slice: mutating the returned slice does not change
   what a later `all()` returns.

Gate: the four commands above.

---

## §B5 — Store and transition tests

Create `internal/poller/store_test.go`. Package `poller`.

Mirror `internal/poller/poller_smoke_test.go`. Drive the store directly with
`NewStore(depth)` and `Record(now, snaps)` — build
`[]backend.UnitSnapshot` literals rather than going through a backend, so each
state change is exactly what you wrote.

The rule being tested, stated plainly: a transition is recorded when a poll sees
an `ActiveState` different from the one the previous poll saw for that unit. The
first observation of a unit is never a transition.

Assert, in prose:

1. A fresh `NewStore(5)` reports `Depth() == 5`, an empty `Names()`, `false`
   from `Latest("anything")`, and `nil` from `History("anything")`.
2. One `Record` of two snapshots yields a sorted two-name `Names()`, one sample
   of history each, and `Counts() == (1, 0)`.
3. The **first** `Record` for a unit returns no transitions. A second `Record`
   with a different `ActiveState` returns exactly one, with the right `Unit`,
   `From`, `To` and `At`. A third `Record` at that same state returns none.
4. With `NewStore(3)` and five recorded polls for one unit, `History` holds
   three samples, oldest first, with increasing `At`.
5. `RecordError` increments the failure count, leaves existing history and the
   `LastPoll` time untouched, and makes `LastPoll`'s error non-nil; a following
   successful `Record` clears that error back to nil.
6. `SnapshotAge` is zero before anything is recorded, and afterwards equals the
   gap between the time passed in and the last recorded poll.

Gate: the four commands above.

---

## §B6 — Poller tests against the fake backend

Create `internal/poller/poller_test.go`. Package `poller`.

Mirror `internal/poller/poller_smoke_test.go` and reuse its `pinnedClock()`
helper. Drive a real fleet with `fake.New()` from
`fleetgauge/internal/backend/fake`, calling `be.Tick()` between polls to make
`flappy.service` change state.

One task-local type to write: a stub backend whose `List` always returns an
error, with the other three methods returning zero values. It needs all four
methods of `backend.Backend` — copy the method signatures from
`internal/backend/backend.go`. That is what assertion 5 needs.

Assert, in prose:

1. `New(be, patterns, 0, 0)` falls back to the defaults: `Interval()` equals
   `DefaultInterval` and the store's `Depth()` equals `DefaultDepth`.
2. `Patterns()` returns a copy — mutating the returned slice does not change
   what a later `Patterns()` returns.
3. `PollOnce` with the single exact pattern `nginx.service` records exactly that
   one unit and no others.
4. Ten `PollOnce` calls with a `be.Tick()` between each leave several
   transitions for `flappy.service` in `Transitions()`, and `wedged.service`
   still reporting `backend.StateFailed`.
5. With the failing stub backend, `PollOnce` returns an error, `Counts()`
   reports zero successful polls and one failure, and `Names()` stays empty.
6. A poller built with depth 3, polled five times, holds three samples for a
   unit — the poll interval and the history depth are independent.

Gate: the four commands above.

---

## §B7 — The overview: the read-side view of the history

Create `internal/poller/overview.go` and `internal/poller/overview_test.go`.
Package `poller`.

This is the shape the page and `/metrics` both consume: one row per unit,
current state plus the history summary. Building it here means neither of them
reaches into the store's internals.

`overview.go` must use **only the Store's exported accessors** (`Names`,
`Latest`, `History`, `Transitions`) and must take no lock of its own — those
accessors already lock, and a second lock around them would deadlock. Do not
edit `store.go`.

Requirements:

1. A `UnitView` struct with: `Name`, `Found`, `ActiveState`, `SubState`,
   `NRestarts`, `MemoryBytes`, `Uptime` (a `time.Duration`), `ObservedAt`
   (a `time.Time`), `Samples` (int), `Transitions` (int).
2. A method `Overview(now time.Time) []UnitView` on `*Store`, returning one view
   per name in `Names()`, in that same order (already sorted).
3. `Uptime` comes from the snapshot's own `Uptime(now)` method — it is already
   defined on `backend.UnitSnapshot`; do not recompute it.
4. `Samples` is how many samples that unit's history holds; `Transitions` is how
   many recorded transitions name that unit.
5. A name in `Names()` with no sample yet yields a view with `Found` false and
   zero values rather than being skipped.

Tests (prose), driven by a fake-backed poller:

- `Overview` returns 12 views for the fake fleet, in sorted name order.
- `wedged.service`'s view reports `backend.StateFailed`.
- After polls with `be.Tick()` between them, `flappy.service`'s view reports a
  nonzero `Transitions` count.
- The unit whose memory is `backend.MemoryUnknown` reports exactly that in its
  view — never 0.
- `Samples` never exceeds the store's `Depth()`.

Gate: the four commands above.

---

## §B8 — The Prometheus text renderer

Create `internal/metrics/metrics.go`. Package `metrics`.

SPEC.md feature 5: per-unit up/state, restart count, memory bytes, snapshot age.
This task is the renderer only — a pure function, no HTTP. The endpoint that
serves it lands in the HTTP phase.

Signature: `Render(st *poller.Store, now time.Time) string`, importing
`fleetgauge/internal/poller` and using `st.Overview(now)` from §B7.

Emit exactly these five families, each preceded by one `# HELP` and one `# TYPE`
line, families in this order, and within a family one line per unit in
`Overview` order:

```
fleetgauge_unit_up{unit="nginx.service"} 1
fleetgauge_unit_state{unit="nginx.service",state="active"} 1
fleetgauge_unit_restarts_total{unit="nginx.service"} 2
fleetgauge_unit_memory_bytes{unit="nginx.service"} 50331648
fleetgauge_snapshot_age_seconds 1.5
```

Rules:

1. `fleetgauge_unit_up` is 1 when `ActiveState` is `backend.StateActive`, else 0.
   Type gauge.
2. `fleetgauge_unit_state` emits one line per unit for its **current** state
   only, with the state as a label value. Type gauge, value always 1.
3. `fleetgauge_unit_restarts_total` is the restart count. Type counter.
4. `fleetgauge_unit_memory_bytes` is **omitted entirely** for a unit whose
   `MemoryBytes` is `backend.MemoryUnknown` — never scrape "accounting is off"
   as a number. Type gauge.
5. `fleetgauge_snapshot_age_seconds` is one unlabelled line, the store's
   `SnapshotAge(now)` in seconds. Type gauge.
6. Label values are escaped Prometheus-style: backslash, double quote and
   newline become `\\`, `\"` and `\n`.

Build the output with a `strings.Builder`. Stdlib only. Mirror the package and
doc-comment style of `internal/poller/store.go` — package comment, one comment
per exported symbol.

Gate: the four commands above.

---

## §B9 — Metrics renderer tests

Create `internal/metrics/metrics_test.go`. Package `metrics`.

Mirror the plain-`testing` style of `internal/poller/poller_smoke_test.go`.
Build a store by constructing a poller over `fake.New()` and calling `PollOnce`,
except where an assertion needs a hand-built snapshot — for those, use
`poller.NewStore` and `Record` directly.

Assert, in prose:

1. Every metric family named in §B8 appears with exactly one `# HELP` line and
   exactly one `# TYPE` line, and the `# TYPE` line for
   `fleetgauge_unit_restarts_total` says `counter` while the others say `gauge`.
2. An active unit renders `fleetgauge_unit_up{unit="nginx.service"} 1`, and
   `wedged.service` renders the same metric with value 0.
3. The fake fleet's `MemoryUnknown` unit produces **no**
   `fleetgauge_unit_memory_bytes` line, and the output contains no memory line
   with a negative value.
4. `fleetgauge_snapshot_age_seconds` appears exactly once and its value parses
   as a non-negative number.
5. Rendering the same store twice returns byte-identical output — ordering is
   deterministic, not map iteration order.
6. A unit whose name contains a double quote renders it escaped as `\"` inside
   the label. Build that one with `NewStore` and `Record` directly.

Gate: the four commands above.

---

## §B10 — Wire `-demo` to the poller in main

Edit `cmd/fleetgauge/main.go`. This is **interim scaffolding**: the HTTP phase
replaces this output with the page. It exists so `go run ./cmd/fleetgauge -demo`
does something real now, and so the whole pipeline is exercised end to end.

Behaviour:

1. With `-demo`: build `fake.New()`, build a poller over it with the pattern
   `*.service`, a 1-second interval and depth 60. Poll five times, calling the
   fake's `Tick()` between polls, then print the overview table and exit 0.
2. Without `-demo`: load the config at `-config` with `config.Load`. On error,
   print it to stderr and exit 1. Otherwise build the systemd backend from
   `fleetgauge/internal/backend/systemd`, build a poller from the config's unit
   names, poll interval and journal-free defaults, `PollOnce` once, print the
   same table, exit 0.
3. The table: one row per `UnitView`, columns Name, State, Restarts, Memory,
   Uptime, Transitions. Align it with `text/tabwriter` from the stdlib. Render
   `backend.MemoryUnknown` memory as the literal `-`, never as a number.
4. Leave the `-addr` flag parsed but unused (`_ = addr`); it belongs to the HTTP
   phase.

Keep the existing flag block in `cmd/fleetgauge/main.go` exactly as it is — its
three flags are the committed surface — and replace only the body below it.
Keep `main` short: a `run() error` helper plus main is the usual shape. Stdlib
only, and no `exec.Command` in this file: `scripts/scrub-check.sh` fails the task
if it appears outside `internal/backend/systemd/`.

Gate: the four commands above, plus `go run ./cmd/fleetgauge -demo` printing a
12-row table.

---

## §B11 — verify.sh, then STATUS.md and ROADMAP.md

No new code. Run the gates, then update the two docs.

1. Run `bash verify.sh`. It must print `verify: all clear` and exit 0. If it does
   not, fix what it reports before touching the docs.
2. **STATUS.md** — append a `## Phase B` section, in the shape of the existing
   `## Phase A` section. Say what shipped: the poller, the per-unit ring-buffer
   history, transition detection, the overview view, the Prometheus text
   renderer, and the interim `-demo` table. Say plainly what is still missing:
   no HTTP surface, so no page, no SSE, and `/metrics` renders but is not served;
   and the real systemd backend is still unproved on live systemd, because
   `scripts/live-check.sh` does not exist and a human runs it.
3. **ROADMAP.md** — flip row 3 to `SHIPPED`, phase `B`, with a one-line note.
   Set row 5 to `PARTIAL`, phase `B`: the renderer is built and tested, the
   endpoint awaits the HTTP surface. Leave every other row alone — row 8 stays
   `PARTIAL` until `-demo` serves the page.
4. In the ROADMAP reservations ledger, record that the `-demo` text table from
   §B10 is interim scaffolding the HTTP phase replaces, so it is not a claim
   that demo mode is done.

Gate: `bash verify.sh` green, plus the four commands above.
