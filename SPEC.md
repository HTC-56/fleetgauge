# fleetgauge — v1 spec

One static Go binary that watches a declared fleet of systemd services and
serves one page that answers "is everything up?" at a glance — plus
Prometheus metrics and a token-gated restart for the units that opt in.
Ops sensibility as a work sample: restart counts, memory, state transitions,
journal tails, and an interface seam that makes systemd itself swappable in
tests. Built end-to-end by an autonomous local-model coding loop; the commit
history is part of the deliverable (see `docs/PROCESS.md` when it lands).

## v1 features (all of these, nothing more)

1. **YAML fleet config.** Units to watch (exact names and globs), listen
   address, bearer token, journal tail depth, poll interval, and per-unit
   `allow_restart: true` opt-in. One file; a commented example ships in
   `deploy/`.
2. **The systemd backend, behind an interface.** Reads unit state via
   `systemctl show` property output and `journalctl` — subprocess calls
   only, no cgo, no D-Bus dependency. Parsed properties: ActiveState,
   SubState, NRestarts, MemoryCurrent, ExecMainStartTimestamp. The backend
   is a narrow Go interface; nothing above it knows systemd exists.
3. **The poller.** Periodic snapshots into a per-unit in-memory ring buffer:
   current state, uptime, restart count, memory, recent state transitions
   with timestamps. No database; history depth is config.
4. **The page.** `GET /` — one self-contained HTML page (inline CSS/JS, no
   framework, no build step, no CDN, no web fonts): unit table with state
   pills, uptime, restarts, memory, a transitions timeline, and a
   journal-tail drawer per unit. Live via SSE. The README hero screenshot.
5. **`/metrics`** — Prometheus text: per-unit up/state, restart count,
   memory bytes, snapshot age.
6. **Restart, gated three ways.** `POST /units/{name}/restart` requires the
   bearer token AND the unit's `allow_restart` opt-in AND appends to a JSONL
   action ledger before executing. Everything else is read-only.
7. **`/healthz` + `/events`** (SSE unit transitions), and structured logs to
   stderr.
8. **Demo mode = the second backend.** `fleetgauge -demo` serves a synthetic
   fleet (a dozen fake units, states drifting, one flapping) from a fake
   backend — the quickstart and hero screenshot need no systemd, and the
   test suite runs against the same fake. The interface seam is
   load-bearing, not decorative.
9. **Deploy-grade packaging.** Single static binary (`CGO_ENABLED=0`);
   Makefile; CI builds linux/amd64 + linux/arm64 and runs the suite; an
   example unit file runs fleetgauge under systemd itself; README quickstart
   (demo mode in one minute, real fleet in five); `docs/PROCESS.md`.

## Pre-registered rules

- Runtime dependency surface: **stdlib + `gopkg.in/yaml.v3`. That is the
  entire list.** A task that adds anything else must name it and why, and
  the default answer is no.
- systemd is reached only through the backend interface; `exec.Command`
  appearing outside the systemd backend package is a spec bug.
- CI runs against the fake backend only. `scripts/live-check.sh` (not CI)
  proves the real backend on a live systemd box and gates any demo claim.
- First Go repo for the build loop: Phase A proves — module builds,
  gofmt/vet/test gates wired, the backend interface + fake backend + one
  parsed `systemctl show` fixture round-trip — before any HTTP surface is
  built. If the executor fails structurally at Go, that is recorded in
  DECISIONS.md and PROCESS.md reports it honestly.

## Non-goals (v1 refuses these)

- No D-Bus, no cgo. No TLS termination (front it with a proxy). No auth
  beyond the static bearer token.
- No multi-host agents, no fleet-of-fleets, no persistence — the ring
  buffer is the history.
- No start/stop/enable/disable of anything, no editing units — restart of
  explicitly opted-in units is the only verb.
- No JS framework, no build step for the page, no websockets (SSE is
  enough).

## Stack & shape

- Go 1.27. Layout: `cmd/fleetgauge/` (main), `internal/` (config, backend,
  backend/systemd, backend/fake, poller, server, page, metrics, ledger),
  `deploy/` (example YAML, example unit file), `scripts/` (scrub-check.sh,
  live-check.sh), `docs/PROCESS.md`.

## Gates

- `gofmt -l .` empty + `go vet ./...` + `go test ./...` green at every
  phase end.
- `bash scripts/scrub-check.sh` green from phase 1: greps the tree for
  private hostnames, non-documentation IPs, absolute home paths, and key
  material. Docs use `localhost` and `192.0.2.x` only.
- `verify.sh` = all of the above + README-quickstart lint (commands shown
  in the README must exist in the repo).

## Done means

A stranger with Go: `go test ./...` green on any OS, no systemd required;
`go run ./cmd/fleetgauge -demo` shows the breathing page with the synthetic
fleet. On a Linux box, the same binary pointed at real units shows real
state, `/metrics` scrapes, and a token-bearing restart of an opted-in unit
lands in the ledger and in the journal (live-check.sh proves it). CI builds
both architectures and the badge is green. PROCESS.md tells the story in
one page.
