# Phase A — the backend seam and the fleet config

**Subject: ROADMAP row 2, "systemd backend behind an interface" (NOT BUILT).**
This phase also carries row 8 (fake backend) and row 1 (YAML fleet config) to
SHIPPED. No HTTP surface is built here — SPEC.md pre-registers that the seam
and the toolchain must be proved first.

**Already shipped by the planning lane** (commits `feat(A1)`–`chore(A3)`) — do
not rewrite these, build against them:

- `internal/backend/backend.go` — `UnitSnapshot`, `ActiveState` constants,
  `MemoryUnknown`, and the four-method `Backend` interface.
- `internal/backend/systemd/` — the real backend + parser + fixture tests.
  **Your pattern file for everything in this phase.**
- `scripts/scrub-check.sh` — the public-repo gate.
- `go.mod`/`go.sum` already require `gopkg.in/yaml.v3`, hashes pinned.
  **Do not run `go mod tidy` before you have written the yaml import** — no
  file imports yaml yet, so tidy would strip the dependency.

**The gate for every task below** (all four, every time; red = not done):

```
gofmt -l .            # must print nothing
go vet ./...
go test ./...
bash scripts/scrub-check.sh
```

---

## §A4 — Fake backend: the static fleet

Create `internal/backend/fake/fake.go`. Package `fake`.

Build a `Backend` struct implementing the `backend.Backend` interface for a
synthetic fleet, so the test suite and `-demo` need no systemd.

Mirror `internal/backend/systemd/systemd.go` for the shape: same four method
signatures, and copy its one-line compile-time assertion that the type
satisfies the interface (the `var _ backend.Backend = ...` line) — that line is
what makes a signature mistake a build error instead of a runtime surprise.

Requirements:

1. `New()` returns a `*Backend` holding a fixed fleet of 12 units with
   plausible service names ending in `.service` (e.g. `nginx.service`,
   `postgres.service`, `worker.service`). One unit **must** be named
   `flappy.service` and one **must** be named `wedged.service`.
2. Starting states: most `active`/`running`; `wedged.service` is
   `failed`/`failed`; give two units a nonzero `NRestarts`.
3. Every unit has `Found: true`, a `LoadState` of `"loaded"`, a `MemoryBytes`
   in a realistic range, and a `StartedAt` in the past. Give **one** unit
   `MemoryBytes: backend.MemoryUnknown` so the "accounting off" path stays
   exercised.
4. `Show` returns snapshots in the same order as the requested names; a name
   that is not in the fleet returns `Found: false` with
   `ActiveState: backend.StateUnknown` and `MemoryBytes: backend.MemoryUnknown`
   (never an error — one missing unit must not blind the page).
5. `List` handles exact names and globs. Use `path.Match` from the stdlib for
   glob patterns; pass non-glob patterns through unchanged. Return names sorted
   and deduplicated.
6. `JournalTail` returns up to `n` plausible synthetic log lines for the unit.
   `Restart` sets the unit `active`/`running` and increments its `NRestarts`.

`StartedAt` must not be computed at package init from a wall clock you cannot
control — take a base time on the struct so tests can pin it.

Guard concurrent access with a `sync.Mutex`: the interface contract says
implementations are safe for concurrent use.

**No `exec.Command` anywhere in this file** — scrub-check will fail the task if
it appears outside `internal/backend/systemd/`.

Gate: the four commands above.

---

## §A5 — Fake backend tests

Create `internal/backend/fake/fake_test.go`. Package `fake`.

Mirror the table-driven, plain-`testing` style of
`internal/backend/systemd/parse_test.go` — no test framework, no new
dependency.

Assert, in prose:

1. `New()` yields exactly 12 units, and `List` with the pattern `*` returns all
   12 names in sorted order.
2. `List` with the exact name `nginx.service` returns exactly that one name;
   `List` with `*.service` returns all 12.
3. `Show` for a name that is not in the fleet returns one snapshot with
   `Found == false` and `MemoryBytes == backend.MemoryUnknown`.
4. `Show` returns snapshots in the **same order as the requested names**, even
   when those names are passed in an order different from the fleet's own.
5. `Restart` on a unit increments that unit's `NRestarts` by exactly one and
   leaves it `active`.
6. Exactly one unit in the starting fleet reports
   `MemoryBytes == backend.MemoryUnknown`.

Gate: the four commands above.

---

## §A6 — Fake backend: drift and the flapping unit

Edit `internal/backend/fake/fake.go` and add tests to
`internal/backend/fake/fake_test.go`.

Give the fake a `Tick()` method that advances the synthetic fleet by one step,
so the demo page visibly breathes. Determinism is the whole point: **no
`math/rand`, no wall-clock reads** — derive everything from an internal step
counter on the struct.

Requirements:

1. `Tick()` increments an unexported step counter.
2. `flappy.service` alternates between `active`/`running` and `failed`/`failed`
   on each tick, and its `NRestarts` increments by one each time it returns to
   `active`.
3. `wedged.service` stays `failed` across every tick — it is the unit that
   never recovers.
4. Units that are `active` have their `MemoryBytes` drift by a small amount
   each tick, staying positive. The unit with `MemoryUnknown` memory keeps
   `MemoryUnknown` — drift must never turn "unknown" into a number.

Tests to add (prose):

- Two fresh fakes ticked the same number of times report identical snapshots —
  determinism.
- After an even number of ticks `flappy.service` is back to its starting state;
  after an odd number it is `failed`.
- `flappy.service`'s `NRestarts` after 10 ticks is greater than after 2 ticks.
- `wedged.service` is still `failed` after 10 ticks.
- The `MemoryUnknown` unit still reports `MemoryUnknown` after 10 ticks.

Gate: the four commands above.

---

## §A7 — The fleet config type and loader

Create `internal/config/config.go`. Package `config`.

This is the first file to import `gopkg.in/yaml.v3` (already in `go.mod`; see
the header note about `go mod tidy`).

The file format — this is the committed shape, from SPEC.md feature 1:

```yaml
listen: "127.0.0.1:8080"
bearer_token: "replace-me"
poll_interval: "5s"
journal_lines: 50
units:
  - name: "nginx.service"
    allow_restart: true
  - name: "worker@*.service"
```

Requirements:

1. A `Config` struct and a `Unit` struct (`Name`, `AllowRestart`) with
   `yaml:"..."` tags matching the keys above.
2. `poll_interval` is a duration string; parse it with `time.ParseDuration`
   into a `time.Duration` field. Keep the raw string in its own yaml-tagged
   field and convert after unmarshal — `time.Duration` does not unmarshal from
   `"5s"` on its own.
3. `Load(path string) (*Config, error)` reads the file, unmarshals, applies
   defaults, then validates. Wrap every returned error with enough context to
   name the file.
4. Defaults when a key is absent: `listen` = `"127.0.0.1:8080"`,
   `poll_interval` = 5 seconds, `journal_lines` = 50.
5. Validation errors (each a distinct, readable message): the units list is
   empty; `poll_interval` is zero or negative; `journal_lines` is negative.

Default `listen` to loopback, never `0.0.0.0` — this thing has one static
bearer token and no TLS.

Gate: the four commands above.

---

## §A8 — Config tests

Create `internal/config/config_test.go`. Package `config`.

Mirror the plain-`testing` style of
`internal/backend/systemd/parse_test.go`. Write YAML fixtures into
`t.TempDir()` and load them by path — do not add testdata files for this task.

Assert, in prose:

1. A full config round-trips: all five top-level keys and a two-unit list load
   with the values written, and `allow_restart` is true only for the unit that
   declared it.
2. A minimal config (only a `units:` list) gets the three defaults from §A7.
3. `poll_interval: "2s"` parses to a 2-second `time.Duration`.
4. A config with an empty units list returns an error.
5. A config with `poll_interval: "0s"` returns an error.
6. Malformed YAML, and a path that does not exist, each return an error rather
   than panicking.

Gate: the four commands above.

---

## §A9 — The example config that ships in deploy/

Create `deploy/fleetgauge.example.yaml`, and add one test to
`internal/config/config_test.go`.

The example is documentation as much as config: a heavily commented file a
stranger can copy and edit. Cover every key from §A7, with a comment above each
explaining what it does and what the default is. Show at least four units, at
least one glob pattern, and exactly one unit with `allow_restart: true` — with
a comment saying that restart also requires the bearer token and that every
restart is appended to the action ledger.

**Public-repo rules apply**: `localhost`/`127.0.0.1` or `192.0.2.x` addresses
only, no real hostnames, and the token value must be an obvious placeholder.

The test: load `../../deploy/fleetgauge.example.yaml` with `config.Load` and
assert it parses without error, yields at least four units, and has exactly one
unit with `AllowRestart == true`. This is what stops the shipped example from
drifting out of sync with the loader.

Gate: the four commands above.

---

## §A10 — verify.sh, then STATUS.md and ROADMAP.md

Create `verify.sh` at the repo root, then update the two docs.

`verify.sh` composes the phase gates into one command. Mirror the structure of
`scripts/scrub-check.sh`: `#!/usr/bin/env bash`, `cd` to the repo root so it
runs from anywhere, run each step, print a clear pass/fail line per step, and
exit non-zero if any step failed. Steps, in order:

1. `gofmt -l .` — fail if it prints anything.
2. `go vet ./...`
3. `go test ./...`
4. `bash scripts/scrub-check.sh`
5. README-quickstart lint: if `README.md` exists, check that every file path it
   mentions inside backticks actually exists in the repo. If `README.md` does
   not exist yet, print that the step is skipped and do **not** fail — the
   README lands in a later phase.

Make it executable (`chmod +x verify.sh`) and stage the mode bit.

Then:

- **STATUS.md** — append a `## Phase A` section: what shipped (the backend
  seam, the systemd parser round-trip, the fake fleet, the config loader), and
  state plainly that no HTTP surface exists yet and that the real backend has
  **not** been proved on live systemd (that needs `scripts/live-check.sh`, which
  a human runs — it does not exist yet either).
- **ROADMAP.md** — flip rows 1 and 2 to `SHIPPED`, phase `A`, one-line note
  each. Row 8 becomes `PARTIAL`, phase `A`: the fake backend exists and is the
  test engine, but `-demo` cannot serve anything until the HTTP surface lands.
  Leave every other row alone. In the reservations ledger, record: the module
  path is the bare `fleetgauge` because the repo name and publish decision are
  human-gated; and that `scripts/live-check.sh` is still unwritten, so no
  live-systemd claim may be made yet.

Gate: `bash verify.sh` green, plus the four commands above.
