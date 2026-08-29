# Phase E — deploy-grade packaging, and the story of the loop

**Subject: ROADMAP row 9, "Deploy-grade packaging" (NOT BUILT), plus the
`docs/PROCESS.md` row.** These are the last two un-SHIPPED rows on the
scoreboard. When this phase lands the project is finished and the planning lane
declares PROJECT SPEC COMPLETE — it does not invent more scope.

No new product behaviour ships here. Nothing in `internal/` changes except one
test. If a task tempts you to add a flag, an endpoint or a config key, the
answer is no: SPEC.md's nine features are all built.

**Already shipped by the planning lane** (commits `feat(E1)`–`fix(E3)`) — do
not rewrite these, build against them:

- `.github/workflows/ci.yml` — two jobs. `gates` runs `verify.sh`; `build`
  cross-compiles linux/amd64 and linux/arm64 with `CGO_ENABLED=0` and checks
  each binary is statically linked.
- `scripts/live-check.sh` — the human-run real-systemd proof. Read-only unless
  the operator passes `--restart NAME`. It refuses to run under CI.
- `verify.sh` — its README lint was broken and is now fixed. See the note
  below; it changes how you must write the README.

**The gate for every task below** (all four, every time; red = not done):

```
gofmt -l .            # must print nothing
go vet ./...
go test ./...
bash scripts/scrub-check.sh
```

**The README lint, once `README.md` exists.** From §E6 on, `bash verify.sh`
also checks that every backticked *repo path* in the README exists. Commands
are fine — `go test ./...` and `bash verify.sh` are skipped by shape — but a
backticked path like `deploy/fleetgauge.service` must be a real file. So never
name a file in the README before the task that creates it has run.

---

## §E4 — the Makefile

Create `Makefile` at the repo root. No Go changes.

**Recipe lines must begin with a literal TAB character, not spaces.** A
Makefile indented with spaces fails with "missing separator". This is the one
way this task usually goes wrong — check it before you run the gate.

Declare every target `.PHONY` and make `build` the default goal.

Targets, each one line or two of recipe:

- `build` — `CGO_ENABLED=0 go build -trimpath -o fleetgauge ./cmd/fleetgauge`.
- `test` — `go test ./...`.
- `vet` — `go vet ./...`.
- `fmt` — `gofmt -w .`.
- `verify` — `bash verify.sh`.
- `demo` — `go run ./cmd/fleetgauge -demo`.
- `dist` — cross-compile both release architectures into `dist/`, producing
  `dist/fleetgauge-linux-amd64` and `dist/fleetgauge-linux-arm64`. Use
  `CGO_ENABLED=0 GOOS=linux GOARCH=<arch>` with `-trimpath` and
  `-ldflags="-s -w"`. Those names and flags must match
  `.github/workflows/ci.yml` — grep its `build static binary` step and copy the
  flags from there so a local `make dist` and CI produce the same artifact.
- `clean` — remove exactly two things: the `fleetgauge` binary and the `dist/`
  directory. Nothing else, ever. Both are gitignored build output.

Put a short comment block at the top saying what the file is for and that
`make verify` is the gate a phase must pass.

Gate: the four commands above, plus `make build` (leaves a `fleetgauge`
binary), `make dist` (leaves both files under `dist/`), and `make verify`
(prints `verify: all clear`). `fleetgauge` and `dist/` are gitignored — stage
the `Makefile` and nothing else.

**Do not run `make demo` to check your work.** It starts a foreground server
and never returns, and the session will hang waiting on it. The three commands
above are the whole gate.

---

## §E5 — the example systemd unit file

Create `deploy/fleetgauge.service`. No Go changes.

This is the unit file that runs fleetgauge under systemd itself — SPEC feature
9. `deploy/fleetgauge.example.yaml` is your pattern file for comment voice:
full sentences above each block, explaining *why*, not just *what*.

Shape it as a normal unit with `[Unit]`, `[Service]` and `[Install]` sections:

- `[Unit]` — a `Description`, and `After=network.target`.
- `[Service]` — `Type=simple`; an `ExecStart` of
  `/usr/local/bin/fleetgauge -config /etc/fleetgauge/fleetgauge.yaml`;
  `Restart=on-failure` with a `RestartSec`.
- `ConfigurationDirectory=fleetgauge` and `StateDirectory=fleetgauge`. The
  second one matters: it makes systemd create `/var/lib/fleetgauge` with the
  right ownership before the process starts, which is where the example
  config's `ledger_path` points. Without it the ledger cannot be opened and
  fleetgauge exits.
- Hardening that does not break the product: `NoNewPrivileges=yes`,
  `PrivateTmp=yes`, `ProtectHome=yes`, `ProtectSystem=full`.
- `[Install]` — `WantedBy=multi-user.target`.

Write a comment saying plainly that the unit runs as **root**, and why: reading
other units' journals and calling `systemctl restart` on them needs privilege.
Note that an operator who wants less can run fleetgauge as a dedicated user
with a polkit rule, and that this file does not try to do that for them.

Add an install comment at the top: copy to `/etc/systemd/system/`, put the
config at `/etc/fleetgauge/fleetgauge.yaml`, then `daemon-reload` and
`enable --now`.

Public-repo rule binds: only `localhost`, `127.0.0.1` or `192.0.2.x` may
appear. No real hostnames.

Gate: the four commands above.

---

## §E6 — README part 1: what it is, and the demo in one minute

Create `README.md` at the repo root. No Go changes.

This is the first half. §E7 writes the second — stop where this section stops.

Read the lint note at the top of this file before you write a single
backtick. `docs/PROCESS.md` does not exist yet: **do not mention it.** §E10
adds that link once the file is there.

Write, in this order:

1. A title and a two-or-three sentence description of what fleetgauge is. The
   opening paragraph of `SPEC.md` is the source — grep it and paraphrase; do
   not copy it whole and do not promise anything it does not.
2. A short "what you need" line: Go 1.27, and no systemd required for the demo
   or the test suite.
3. **Demo in one minute.** `go run ./cmd/fleetgauge -demo`, then open
   `http://127.0.0.1:8080`. Say what the operator sees: a synthetic twelve-unit
   fleet with states drifting and one unit flapping, live over SSE. Mention
   `-addr` for a different listen address. Mention that demo mode is read-only
   and draws no restart buttons unless `-token` is passed, and that this is
   deliberate — an unattended demo must not be able to mutate anything.
4. **The endpoints**, as a small markdown table with a one-line description
   each: `GET /` the page, `GET /healthz` (503 until the first poll lands),
   `GET /metrics` Prometheus text, `GET /events` the SSE stream,
   `GET /units/{name}/journal` the journal drawer, and
   `POST /units/{name}/restart` the only mutating verb.

There is no screenshot in this repo and you cannot make one. Describe the page
in prose; do not write an image tag or a TODO for one.

Gate: the four commands above, plus `bash verify.sh` — the README lint is now
live, and this task is the first thing that runs it.

---

## §E7 — README part 2: a real fleet in five minutes

Edit `README.md`, appending to what §E6 wrote. No Go changes.

Same lint rule: every backticked repo path must exist. `docs/PROCESS.md` still
does not — do not mention it.

Append these sections:

1. **A real fleet in five minutes.** Copy `deploy/fleetgauge.example.yaml`,
   edit the unit list and the token, and run with `-config`. Say that every key
   is documented in that example file rather than repeating the key list here —
   two copies of the same list will drift.
2. **Running it under systemd.** Point at `deploy/fleetgauge.service` and give
   the four steps its header comment describes: install the binary, install the
   config, install the unit, enable it. Note `StateDirectory=fleetgauge` is what
   creates the ledger's home.
3. **Restart is gated three ways** — a short paragraph: the bearer token, the
   unit's own `allow_restart` opt-in, and an append-only JSONL ledger line
   written *before* the backend is touched. Say that a config with an
   `allow_restart` unit and no token fails to load.
4. **Building and testing.** `make build`, `make test`, `make dist` for both
   release architectures, and `bash verify.sh` as the full gate. Mention CI runs
   the same `verify.sh`.
5. **What is not proved.** This section is not optional and must not be softened.
   The test suite runs entirely against the fake backend, which is what lets it
   pass on any OS with no systemd. That means no green build proves the real
   systemd backend. `scripts/live-check.sh` is the only thing that does, a human
   runs it on a real systemd box, and **at the time of writing nobody has.**
   State that plainly. Do not write that fleetgauge has been run against real
   units — it is the one claim this repo is not allowed to make.

Gate: the four commands above, plus `bash verify.sh`.

---

## §E8 — the unit file cannot drift from the example config

Edit `internal/config/config_test.go` only. Add one test function; change
nothing that is already there.

`TestConfigExampleLoad` (near the bottom of that file) is your pattern: it
loads `../../deploy/fleetgauge.example.yaml` with `Load` and asserts on the
result, so the shipped example can never drift from the loader. Two deploy
artifacts now have to agree with each other in the same way, and nothing
checks it: the unit file's `StateDirectory` creates the directory the example
config's `ledger_path` writes into. If either moves, fleetgauge fails to start
on a real box — and every test still passes.

Write one new test that reads `../../deploy/fleetgauge.service` with
`os.ReadFile` and asserts, in prose:

1. The file exists and is non-empty.
2. It has all three section headers — `[Unit]`, `[Service]` and `[Install]`.
3. Its `ExecStart` line names a `-config` flag. (Find the line with
   `strings.Contains`; the point is that the shipped unit passes a config path
   at all.)
4. It sets `StateDirectory=fleetgauge`.
5. The example config's `LedgerPath` — read it by calling `Load` on
   `../../deploy/fleetgauge.example.yaml`, do not hard-code the string — is an
   absolute path under the directory that `StateDirectory` creates,
   `/var/lib/fleetgauge/`. This is the assertion that does the real work: it is
   what fails if someone moves the ledger without moving the unit file.

Use `strings` and `os` only. No new imports beyond those and what the file
already has.

Gate: the four commands above.

---

## §E9 — [CLAUDE] docs/PROCESS.md — the story the ledger tells

Create `docs/PROCESS.md`. Reserved for the review agent: it requires reading
`loop-ledger.tsv` and deciding honestly what the numbers mean, including the
null results, which is exactly the judgment the local lane should not be asked
to make about its own performance.

One page. What it must cover: how the loop is built (a planning lane that
writes phases and implements what the local model cannot, a local 35B executor
that carries the tasks, `verify.sh` as the shared gate); what actually happened
across phases A–E, with real numbers excerpted from `loop-ledger.tsv`, not
rounded impressions; where the local model succeeded and where it did not,
naming the `result=no-op` rows rather than quietly averaging them away; and
what the split between planner-authored and executor-authored commits really
was, since an honest account of that split is the whole point of the document.

It must repeat the reservation the README carries: the real systemd backend is
unproved, `scripts/live-check.sh` is the only proof and a human has not run it.

Public-repo rule binds, including the no-private-project rule that
`scripts/scrub-check.sh` enforces.

Gate: the four commands above, plus `bash verify.sh`.

---

## §E10 — verify.sh, then STATUS.md and ROADMAP.md

No new code. Run the gates, then update the docs. This is the last task in the
project.

1. Run `bash verify.sh`. It must print `verify: all clear` and exit 0. If it
   does not, fix what it reports before touching the docs.
2. **README.md** — add one line near the end linking `docs/PROCESS.md` as the
   story of how the repo was built. §E9 created it, so the lint will now
   accept the path. If `docs/PROCESS.md` does not exist yet, skip this step and
   say so in your commit message rather than inventing the file.
3. **STATUS.md** — append a `## Phase E` section shaped like `## Phase D`. What
   shipped: the Makefile, the CI workflow with dual-arch static builds, the
   example systemd unit file, the README with both quickstarts, the unit-file
   drift test, and `scripts/live-check.sh`. What is still not true: no human has
   run `live-check.sh`, so the real systemd backend remains unproved and no
   claim about it may appear anywhere; and there is no hero screenshot.
4. **ROADMAP.md** — flip row 9 to `SHIPPED`, phase `E`, with a one-line note,
   and the `docs/PROCESS.md` row to `SHIPPED`, phase `E`. Every row should now
   read SHIPPED.
5. In the reservations ledger, record two things: that the hero screenshot for
   the page was never produced, because it needs a browser and a human and the
   loop has neither, so the README describes the page in prose instead; and
   that `live-check.sh` now exists but has not been run by a human, which
   leaves the real-systemd claim unmade on purpose.

Gate: `bash verify.sh` green, plus the four commands above.
