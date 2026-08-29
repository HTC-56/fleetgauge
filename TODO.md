# Loop tasks

Ordered; each is one short session. Work the first unchecked box. Each task is
fully specced in ONE greppable section of its phase doc (`TASK_PHASE_A.md` §A1,
§A2, …) — grep your section, read it, build it.

*(no tasks yet — the planning lane authors Phase A from SPEC.md)*

## Phase A: the backend seam and the fleet config — see TASK_PHASE_A.md

The planning lane already shipped the backend interface, the systemd backend +
parser, and `scripts/scrub-check.sh` (commits `feat(A1)`–`chore(A3)`). Build
against those; `internal/backend/systemd/` is your pattern file for all of it.
Gate for every task: `gofmt -l .` empty, `go vet ./...`, `go test ./...`,
`bash scripts/scrub-check.sh`.

- [x] §A4 — Create `internal/backend/fake/fake.go`: a `Backend` for a static
  12-unit synthetic fleet, implementing the `backend.Backend` interface.
  Mirror `internal/backend/systemd/systemd.go`. Spec: TASK_PHASE_A.md §A4.
- [x] §A5 — Create `internal/backend/fake/fake_test.go`: 6 assertions on the
  static fleet (count, glob List, missing unit, Show ordering, Restart).
  Mirror `internal/backend/systemd/parse_test.go`. Spec: §A5.
- [x] §A6 — Edit `internal/backend/fake/fake.go`: add a deterministic `Tick()`
  (no rand, no clock) so `flappy.service` flaps and `wedged.service` stays
  failed; add the 5 drift tests. Spec: §A6.
- [x] §A7 — Create `internal/config/config.go`: the YAML fleet config struct,
  `Load`, three defaults and three validation errors. First file to import
  `gopkg.in/yaml.v3` — read the §A7 note on `go mod tidy`. Spec: §A7.
- [x] §A8 — Create `internal/config/config_test.go`: 6 assertions using YAML
  fixtures written into `t.TempDir()` — round-trip, defaults, duration
  parsing, and three error cases. Spec: §A8.
- [x] §A9 — Create `deploy/fleetgauge.example.yaml`, a commented example
  covering every key, plus one test in `config_test.go` that loads it so the
  example cannot drift from the loader. Spec: §A9.
- [x] §A10 — Create `verify.sh` (gofmt + vet + test + scrub + README lint),
  then append a Phase A section to STATUS.md and update ROADMAP rows 1 and 2 to
  SHIPPED, row 8 to PARTIAL, with the two reservations. Spec: §A10.

## Phase B: the poller, the history, and the metrics text — see TASK_PHASE_B.md

The planning lane already shipped `internal/poller/` — `ring.go`, `store.go`,
`poller.go` and a smoke test (commits `feat(B1)`–`feat(B3)`). Build against
those; `internal/poller/poller_smoke_test.go` is your pattern file for every
test task, and it defines `pinnedClock()` — reuse it, never redefine it.
Gate for every task: `gofmt -l .` empty, `go vet ./...`, `go test ./...`,
`bash scripts/scrub-check.sh`.

- [x] §B4 — Create `internal/poller/ring_test.go`: 6 assertions on the
  unexported ring (capacity, oldest-first order, overwrite on wrap, `last()`,
  zero capacity, fresh slice). Spec: TASK_PHASE_B.md §B4.
- [x] §B5 — Create `internal/poller/store_test.go`: 6 assertions driving
  `Record`/`RecordError` directly — first observation is not a transition,
  depth cap, snapshot age, error handling. Spec: §B5.
- [x] §B6 — Create `internal/poller/poller_test.go`: 6 assertions against
  `fake.New()` plus one task-local always-failing stub backend. Spec: §B6.
- [x] §B7 — Create `internal/poller/overview.go` + `overview_test.go`:
  `UnitView` and `Store.Overview(now)`, built only from the Store's exported
  accessors, no new locking. Spec: §B7.
- [x] §B8 — Create `internal/metrics/metrics.go`: `Render(store, now) string`,
  the five Prometheus families from SPEC feature 5. Memory is omitted, never
  negative, when accounting is off. Spec: §B8.
- [x] §B9 — Create `internal/metrics/metrics_test.go`: 6 assertions — HELP/TYPE
  lines, up 1 vs 0, no memory line for the unknown unit, byte-identical
  re-render, escaped label. Spec: §B9.
- [x] §B10 — Edit `cmd/fleetgauge/main.go`: `-demo` polls the fake fleet five
  times and prints a `text/tabwriter` overview table; no `-demo` loads the
  config and polls once. Interim until the page lands. Spec: §B10.
- [x] §B11 — Run `bash verify.sh`, append a Phase B section to STATUS.md, flip
  ROADMAP row 3 to SHIPPED and row 5 to PARTIAL, and record the interim
  `-demo` table as a reservation. Spec: §B11.

## Phase C: the HTTP surface — see TASK_PHASE_C.md

The planning lane already shipped `internal/server/` — `hub.go`, `fleet.go`,
`sse.go`, `doc.go` — and `internal/page/` (commits `feat(C1)`–`feat(C3)`).
Build against those; `internal/server/server_smoke_test.go` is your pattern
file for every test task, and it defines `pinnedClock()`, `newTestServer()`
and `waitFor()` — reuse them, never redefine them. `doc.go` already holds the
package comment; do not write a second one.
Gate for every task: `gofmt -l .` empty, `go vet ./...`, `go test ./...`,
`bash scripts/scrub-check.sh`.

- [x] §C4 — Create `internal/server/log.go` + `log_test.go`: a
  `LogRequests(log, next)` slog middleware logging method/path/status/
  duration_ms. Its ResponseWriter wrapper must implement `http.Flusher` or
  `/events` breaks. 5 assertions. Spec: TASK_PHASE_C.md §C4.
- [x] §C5 — Create `internal/server/server.go`: `Handler()` wiring a ServeMux
  for `GET /{$}`, `/healthz`, `/metrics`, `/events`, wrapped in `LogRequests`,
  plus the three thin handlers. `/healthz` is 503 when unpolled. Spec: §C5.
- [x] §C6 — Create `internal/server/server_test.go`: 6 httptest assertions on
  the four routes — page HTML, metrics text, healthz ok vs degraded, 404 on an
  unknown path, 405 on POST. Spec: §C6.
- [x] §C7 — Create `internal/server/journal.go` + `journal_test.go` and add the
  `GET /units/{name}/journal` route to `Handler()`. 400/503/502 error paths;
  lines marshal as `[]`, never `null`. 4 assertions. Spec: §C7.
- [x] §C8 — Create `internal/server/hub_test.go`: 6 assertions on the hub and
  `/events` — two subscribers, double cancel, drop-on-slow, Close semantics,
  context cancel, transition events. Spec: §C8.
- [x] §C9 — Edit `cmd/fleetgauge/main.go`: both modes serve HTTP instead of
  printing the table. Poller and `Broadcast` in goroutines, `signal.NotifyContext`,
  graceful `Shutdown`. Drop the tabwriter helpers. Spec: §C9.
- [x] §C10 — Run `bash verify.sh`, append a Phase C section to STATUS.md, flip
  ROADMAP rows 4, 5, 7 and 8 to SHIPPED, and record the two new reservations.
  Spec: §C10.

## Phase D: the triple-gated restart — see TASK_PHASE_D.md

The planning lane already shipped `internal/ledger/ledger.go`,
`internal/server/restart.go`, the two new `Options` fields, the route, and the
page's restart control (commits `feat(D1)`–`feat(D3)`). Build against those.
`internal/server/server_test.go` is your pattern file for HTTP test tasks and
`internal/config/config_test.go` for file-writing ones; both reuse helpers that
already exist — never redefine `pinnedClock()`.
Gate for every task: `gofmt -l .` empty, `go vet ./...`, `go test ./...`,
`bash scripts/scrub-check.sh`.

- [x] §D4 — Create `internal/ledger/ledger_test.go`: 6 assertions on the JSONL
  file (create, one object per line, field round-trip, reopen appends, Append
  after Close, 20 concurrent appends). Spec: TASK_PHASE_D.md §D4.
- [x] §D5 — Create `internal/server/restart_test.go`: 6 assertions on the three
  gates — 503 unconfigured, 401 bad token, 403 not opted in, 200 plus two
  ledger lines, failing ledger refuses, 405 on GET. Spec: §D5.
- [x] §D6 — Edit `internal/config/config.go`: add the `ledger_path` key with
  default `ledger.jsonl`, plus one rule — `allow_restart` needs a
  `bearer_token`. 3 tests in `config_test.go`. Spec: §D6.
- [x] §D7 — Edit `deploy/fleetgauge.example.yaml`: document `ledger_path`, fix
  the `journal_lines` comment, extend the `bearer_token` comment with the new
  rule. No Go changes. Spec: §D7.
- [x] §D8 — Edit `cmd/fleetgauge/main.go`: open the ledger and pass
  `BearerToken` + `Ledger` in both modes; add a `-token` flag that turns demo
  restart on for three units. Spec: §D8.
- [x] §D9 — Create `internal/page/page_test.go`: 6 assertions that the page
  stays self-contained (no http://, no `<link`, no @import) and carries the
  restart control and a 7-wide drawer. Spec: §D9.
- [x] §D10 — Run `bash verify.sh`, append a Phase D section to STATUS.md, flip
  ROADMAP row 6 to SHIPPED, and record the two new reservations. Spec: §D10.

## Phase E: deploy-grade packaging — see TASK_PHASE_E.md

The last phase. The planning lane already shipped `.github/workflows/ci.yml`,
`scripts/live-check.sh`, and a fix to `verify.sh`'s README lint (commits
`feat(E1)`–`fix(E3)`). No new product behaviour ships here — SPEC.md's nine
features are all built. From §E6 on, `bash verify.sh` also checks that every
backticked repo path in the README exists; read the lint note at the top of
TASK_PHASE_E.md before writing any README.
Gate for every task: `gofmt -l .` empty, `go vet ./...`, `go test ./...`,
`bash scripts/scrub-check.sh`.

- [x] §E4 — Create `Makefile`: build/test/vet/fmt/verify/demo/dist/clean,
  default goal `build`, `CGO_ENABLED=0`. Recipe lines need literal TABs.
  Spec: TASK_PHASE_E.md §E4.
- [x] §E5 — Create `deploy/fleetgauge.service`: the example systemd unit, with
  `StateDirectory=fleetgauge` and four hardening keys. Mirror the comment voice
  of `deploy/fleetgauge.example.yaml`. Spec: §E5.
- [x] §E6 — Create `README.md`: what it is, the one-minute demo quickstart, and
  the endpoint table. First task the README lint gates. Spec: §E6.
- [x] §E7 — Edit `README.md`: real fleet in five, the systemd install, the three
  restart gates, build and test, and the "what is not proved" section.
  Spec: §E7.
- [x] §E8 — Edit `internal/config/config_test.go`: one test that
  `deploy/fleetgauge.service`'s `StateDirectory` agrees with the example
  config's `ledger_path`. Mirror `TestConfigExampleLoad`. Spec: §E8.
- [ ] §E9 — [CLAUDE] Create `docs/PROCESS.md`: the loop story, with real
  numbers excerpted from `loop-ledger.tsv` and the null results named.
  Spec: §E9.
- [ ] §E10 — Run `bash verify.sh`, append a Phase E section to STATUS.md, flip
  ROADMAP rows 9 and `docs/PROCESS.md` to SHIPPED, record two reservations.
  Spec: §E10.
