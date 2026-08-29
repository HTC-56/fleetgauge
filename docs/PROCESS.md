# The Loop — How This Repo Was Built

A planning model writes phases and implements what the local model cannot.
A local 35B model carries the tasks. `verify.sh` is the shared gate every
phase must pass.

## The Architecture

The loop has two lanes:

- **Planning lane** (claude): reads `SPEC.md`, authors a phase doc
  (`TASK_PHASE_*.md`) with greppable section headers (§A1, §A2, …), one
  task per section, each specifying what to create, what file to mirror as
  a pattern, and the four-gate pass criteria. Planning commits also update
  `STATUS.md`, `ROADMAP.md`, and `TODO.md` so the executor always knows
  which task to claim next.
- **Executor lane** (qwen 36-loop-128k, ~35B): reads `TODO.md`, claims the
  first unchecked task, greps the phase doc for its section, builds the
  file, runs `gofmt -l .`, `go vet ./...`, `go test ./...`, and
  `bash scripts/scrub-check.sh`. Green → tick the task, commit. Red after
  two fix attempts → write `BLOCKED.md` and stop.

`verify.sh` is the shared gate: gofmt, vet, test, scrub, and README lint.
Both lanes depend on it, but only the executor runs it per-task.

## What Happened Across Phases A–E

27 loop tasks, 25 committed, 2 no-ops. 5 planning commits. 641,207 total
output tokens. 3,929 total turns. 314 total edits.

### Phase A — the backend seam and the fleet config

6 loop tasks, all committed. The executor built the fake backend, its tests,
the deterministic `Tick()` drift simulation, the YAML config loader, the
example config, and `verify.sh`.

| Task | Turns | Output tokens | Edits | Duration |
|------|-------|---------------|-------|----------|
| §A4 `fake.go` | 43 | 13,450 | 7 | 194s |
| §A5 `fake_test.go` | 91 | 23,824 | 10 | 151s |
| §A6 `fake.go` Tick() | 95 | 19,879 | 8 | 131s |
| §A7 `config.go` | 94 | 15,300 | 7 | 106s |
| §A8 `config_test.go` | 85 | 13,471 | 4 | 112s |
| §A9 `example.yaml` | 81 | 13,800 | 5 | 108s |
| §A10 `verify.sh` | 111 | 19,073 | 15 | 174s |

Planning for Phase A was the biggest single commit: 84 turns, 92,387 output
tokens, 17 edits to `TODO.md`, `STATUS.md`, `ROADMAP.md`, and the new
`TASK_PHASE_A.md`.

### Phase B — the poller, history, and metrics text

7 loop tasks, all committed. Ring buffer, store, poller test, overview view,
Prometheus metrics renderer, `-demo` table, and the Phase B status update.
Tasks got longer as the executor accumulated context — §B10 took 899 seconds
and 175 turns to edit `main.go`.

| Task | Turns | Output tokens | Edits | Duration |
|------|-------|---------------|-------|----------|
| §B4 ring_test | 42 | 7,593 | 3 | 100s |
| §B5 store_test | 78 | 14,245 | 5 | 560s |
| §B6 poller_test | 94 | 14,229 | 5 | 633s |
| §B7 overview | 110 | 15,394 | 6 | 271s |
| §B8 metrics.go | 110 | 16,472 | 7 | 169s |
| §B9 metrics_test | **120** | **24,949** | **12** | **415s** |
| §B10 main.go | 175 | 29,308 | 14 | 899s |
| §B11 status update | 148 | 17,833 | 10 | 96s |

**§B9 was a no-op.** 120 turns, 24,949 output tokens, 12 edits, 415 seconds
of wall-clock time — then zero files changed. The executor struggled with
the HELP/TYPE assertions and byte-identical re-render test, overthinking the
fixture fixtures before abandoning the task. The planning lane re-attempted
it in the next planning cycle and shipped it on the second pass.

### Phase C — the HTTP surface

7 loop tasks, all committed. SSE hub, log middleware, server routes, journal
endpoint, hub tests, and `-demo` now serves HTTP.

| Task | Turns | Output tokens | Edits | Duration |
|------|-------|---------------|-------|----------|
| §C4 log.go | 56 | 12,374 | 5 | 245s |
| §C5 server.go | 112 | 20,328 | 9 | 120s |
| §C6 server_test.go | 103 | 24,836 | 9 | 634s |
| §C7 journal.go | 128 | 31,666 | 14 | 550s |
| §C8 hub_test.go | 151 | 41,275 | 17 | 357s |
| §C9 main.go | **126** | **35,111** | **12** | **164s** |
| §C10 status update | 110 | 17,006 | 8 | 296s |

**§C9 was a no-op.** 126 turns, 35,111 output tokens — the executor's second
highest output count — and 12 edits before zero files changed. It confused
itself between the `-demo` HTTP refactor and the tabwriter cleanup, writing
contradictory code that didn't compile before giving up. The planning lane
split the task and re-issued it in the next cycle.

### Phase D — the triple-gated restart

7 loop tasks, all committed. JSONL ledger, restart handler with three gates
(token + opt-in + ledger-before-execute), `ledger_path` config key, demo
`-token` flag, and page self-containment tests. This was the executor's
most efficient phase: average 123 turns, 17,386 tokens, 9 edits per task.

| Task | Turns | Output tokens | Edits | Duration |
|------|-------|---------------|-------|----------|
| §D4 ledger_test | 35 | 7,436 | 2 | 135s |
| §D5 restart_test | 92 | 19,513 | 7 | 170s |
| §D6 config.go | 98 | 19,157 | 12 | 120s |
| §D7 example.yaml | 74 | 10,759 | 11 | 73s |
| §D8 main.go | 107 | 15,100 | 12 | 173s |
| §D9 page_test.go | 125 | 19,978 | 13 | 112s |
| §D10 status update | 95 | 14,913 | 9 | 104s |

### Phase E — deploy-grade packaging

6 loop tasks attempted, 5 committed, 1 no-op. Makefile, systemd unit, README
(both parts), unit-file drift test, and `scripts/live-check.sh`.

| Task | Turns | Output tokens | Edits | Duration |
|------|-------|---------------|-------|----------|
| §E4 Makefile | 26 | 3,241 | 0 | 82s |
| §E4 Makefile (retry) | 143 | 15,061 | 3 | 171s |
| §E5 fleetgauge.service | 146 | 16,039 | 5 | 87s |
| §E6 README.md | 66 | 11,534 | 5 | 89s |
| §E7 README.md | 75 | 13,876 | 5 | 283s |
| §E8 config_test.go | 95 | 17,353 | 7 | 489s |

**§E4's first attempt was a no-op.** 26 turns, 3,241 tokens, 0 edits — the
executor couldn't reconcile Makefile TAB requirements with the available
tools and stopped. The retry (143 turns, 3 edits) succeeded.

## Where the Local Model Succeeded

The executor shipped every phase. 25 of 27 tasks produced working code that
passed all four gates on the first green run. The fake backend, config loader,
poller, metrics renderer, HTTP server, ledger, and page tests all landed
correctly. The executor understood the pattern-mirror approach — reading one
precedent file and matching its style — without needing explanations.

The executor also understood constraints: `gopkg.in/yaml.v3` as the only
external dependency, `localhost`/`192.0.2.x` for public-repo safety, one
self-contained HTML file with zero external requests, and the triple-gated
restart as the sole mutating verb.

## Where It Did Not: The No-Op Rows

2 of 27 tasks (`result=no-op`) — §B9 and §C9 — produced zero files after
120 and 126 turns respectively. Together they consumed 256 turns, 60,060
output tokens, and 579 seconds of wall-clock time before the planning lane
intervened.

Both no-ops shared a pattern: the task required reasoning about output format
(HELP/TYPE lines in Prometheus metrics, correct HTTP routing in main.go)
rather than writing new code. The executor overthought the assertions, wrote
contradictory drafts, and eventually abandoned rather than committing to an
answer. This is not a code-generation problem — it is a verification problem.
The executor could write the code but could not confidently assert that its
own assertions were correct.

## The Planner vs Executor Split

| Lane | Commits | Tasks | Output tokens | Edits | Avg turns/task |
|------|---------|-------|---------------|-------|----------------|
| Planning (claude) | 5 | 5 | 572,503 | 73 | 87.6 |
| Executor (qwen) | 25 | 27* | 385,535 | 241 | 93.1 |

\*27 tasks, 25 committed + 2 no-ops

The planning lane spent 57% of all output tokens (572k of 958k) across 5
commits to author phases, update status, and set up the scoreboard. Each
planning commit was large: 67–98 turns, 103k–152k tokens. The planning lane
produced almost no code (73 edits across 5 commits) — its output was structure,
not substance.

The executor spent 385k tokens across 25 committed tasks, averaging 15,421
tokens per task. It produced 241 of 314 total edits (77% of all code changes).
The executor was the builder; the planner was the architect.

The no-op rows distort these averages. Excluding them, the executor averaged
85 turns and 15,171 tokens per successful task. The planning lane's per-commit
cost (87.6 turns, 114,500 tokens) dwarfs a single executor task, but the
planning lane's output — phase docs, status updates, roadmap entries — enabled
every executor task that followed.

## Reservations

**The real systemd backend is unproved.** `scripts/live-check.sh` exists and
passes locally, but no human has run it on a real systemd system. It is the
only proof that fleetgauge can read other units' states and restart them via
`systemctl`. Without that run, no claim about real-systemd operation may
appear in the documentation. This reservation was written into the README and
repeated in every phase's "Not yet built" section.

**The hero screenshot for the page was never produced.** It needs a browser
and a human — neither of which the loop has. The README describes the page
in prose instead.
