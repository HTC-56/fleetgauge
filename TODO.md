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
- [ ] §B5 — Create `internal/poller/store_test.go`: 6 assertions driving
  `Record`/`RecordError` directly — first observation is not a transition,
  depth cap, snapshot age, error handling. Spec: §B5.
- [ ] §B6 — Create `internal/poller/poller_test.go`: 6 assertions against
  `fake.New()` plus one task-local always-failing stub backend. Spec: §B6.
- [ ] §B7 — Create `internal/poller/overview.go` + `overview_test.go`:
  `UnitView` and `Store.Overview(now)`, built only from the Store's exported
  accessors, no new locking. Spec: §B7.
- [ ] §B8 — Create `internal/metrics/metrics.go`: `Render(store, now) string`,
  the five Prometheus families from SPEC feature 5. Memory is omitted, never
  negative, when accounting is off. Spec: §B8.
- [ ] §B9 — Create `internal/metrics/metrics_test.go`: 6 assertions — HELP/TYPE
  lines, up 1 vs 0, no memory line for the unknown unit, byte-identical
  re-render, escaped label. Spec: §B9.
- [ ] §B10 — Edit `cmd/fleetgauge/main.go`: `-demo` polls the fake fleet five
  times and prints a `text/tabwriter` overview table; no `-demo` loads the
  config and polls once. Interim until the page lands. Spec: §B10.
- [ ] §B11 — Run `bash verify.sh`, append a Phase B section to STATUS.md, flip
  ROADMAP row 3 to SHIPPED and row 5 to PARTIAL, and record the interim
  `-demo` table as a reservation. Spec: §B11.
