# Status

Repo scaffolded 2026-08-27. Nothing built yet. SPEC.md is the product;
DECISIONS.md locks the fence; ROADMAP.md is the scoreboard. The planning lane
authors Phase A from SPEC.md (Phase A must prove the toolchain — gofmt, vet,
test — plus the backend interface, the fake backend, and one parsed
`systemctl show` fixture round-trip before any HTTP surface is built; this
is the executor's first Go repo).

Per-phase sections append below as phases ship.

## Phase B

**Shipped:** the poller with per-unit ring-buffer history, transition detection,
the overview view (`Store.Overview`), the Prometheus text renderer
(`metrics.Render`), and the interim `-demo` table that polls the fake fleet
and prints a `text/tabwriter` overview. Tests cover the ring, store, poller,
overview, and metrics renderer (6 assertions each).

**Not yet built:** no HTTP surface, so no page, no SSE, and `/metrics` renders
but is not served; and the real systemd backend is still unproved on live
systemd — `scripts/live-check.sh` does not exist and a human runs it.

## Phase A

**Shipped:** the backend seam (interface + systemd backend + systemctl parse
pipeline), the fake synthetic fleet (`fake.go` with deterministic `Tick()` and
drift simulation), the YAML fleet config loader (`config.go` with
`Load()`/defaults/validation), and `deploy/fleetgauge.example.yaml`. Tests
cover the fake backend, config round-trips, and scrub-check hygiene.

**Not yet built:** no HTTP surface, no SSE, no `/metrics`, no `/healthz`.
The real systemd backend has not been proved on live systemd — that requires
`scripts/live-check.sh`, which a human runs and does not exist yet.

## Phase C

**Shipped:** the SSE hub and broadcast loop, the self-contained HTML page,
`/events` (SSE stream), `/healthz` (503 when unpolled, 200 once polled),
`/metrics` (Prometheus text served from the poller store), the
`/units/{name}/journal` drawer endpoint, structured request logs via
`LogRequests` middleware, and `-demo` now serving the page instead of
printing a table. Tests cover the log middleware, server routes, journal
endpoint, and hub/event semantics (4–6 assertions each).

**Not yet built:** the triple-gated restart and its action ledger are not
built, so the product is still entirely read-only; there is no README, no
Makefile and no CI; and the real systemd backend is still unproved on live
systemd, because `scripts/live-check.sh` does not exist and a human runs it.
