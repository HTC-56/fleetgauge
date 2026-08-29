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

**Not yet built:** there is no README, no Makefile and no CI; and the real
systemd backend is still unproved on live systemd, because
`scripts/live-check.sh` does not exist and a human runs it.

## Phase D

**Shipped:** the append-only JSONL action ledger, the triple-gated
`POST /units/{name}/restart` (token, per-unit opt-in, ledger-before-execute),
the `ledger_path` config key and the rule that an `allow_restart` unit
requires a bearer token, the restart control on the page, and demo mode's
`-token` flag. Tests cover the ledger, restart handler, and page
self-containment (6 assertions each).

**Not yet built:** there is no README, no Makefile and no CI; and the real
systemd backend is still unproved on live systemd, because
`scripts/live-check.sh` does not exist and a human runs it.

## Phase E

**Shipped:** the Makefile (`build`/`test`/`dist`/`clean`, `CGO_ENABLED=0`), the
CI workflow with dual-arch static builds, the example systemd unit file with
`StateDirectory` and hardening keys, the README with both quickstarts and the
restart-gate section, the unit-file drift test between `fleetgauge.service` and
`fleetgauge.example.yaml`, and `scripts/live-check.sh`.

**Not yet true:** no human has run `live-check.sh` on a real systemd box, so the
real systemd backend remains unproved and no claim about it appears in the docs;
and the hero screenshot for the page was never produced — it needs a browser and
a human, neither of which the loop has.

## Human proofs (2026-08-29)

Both "Not yet true" items from Phase E are now true, done by hand outside the
loop.

**`live-check.sh` on a real systemd box** — read-only run, all clear:

    === verdict ===
      6 passed, 0 failed
      host:  systemd 255 (255.4-1ubuntu8.16)
      unit:  systemd-journald.service

    checks: GET / served the page; /metrics reported the unit;
    parsed ActiveState matched systemctl (active); the journal drawer
    returned lines; restart without a token was 401; restart of a
    non-opted-in unit was 403.

    live-check: all clear on real systemd.

On that evidence the real-systemd claim is written down. The mutating restart
path was deliberately not exercised — the run was read-only; the script's
`--restart` opt-in stays unused until someone names a unit they mean it for.

**The hero screenshot** — `docs/hero.png`, taken from `-demo` (served with
`-addr 127.0.0.1:8083`; the default :8080 was occupied on the box). Referenced
from the README.
