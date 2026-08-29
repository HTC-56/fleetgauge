# Status

Repo scaffolded 2026-08-27. Nothing built yet. SPEC.md is the product;
DECISIONS.md locks the fence; ROADMAP.md is the scoreboard. The planning lane
authors Phase A from SPEC.md (Phase A must prove the toolchain — gofmt, vet,
test — plus the backend interface, the fake backend, and one parsed
`systemctl show` fixture round-trip before any HTTP surface is built; this
is the executor's first Go repo).

Per-phase sections append below as phases ship.

## Phase A

**Shipped:** the backend seam (interface + systemd backend + systemctl parse
pipeline), the fake synthetic fleet (`fake.go` with deterministic `Tick()` and
drift simulation), the YAML fleet config loader (`config.go` with
`Load()`/defaults/validation), and `deploy/fleetgauge.example.yaml`. Tests
cover the fake backend, config round-trips, and scrub-check hygiene.

**Not yet built:** no HTTP surface, no SSE, no `/metrics`, no `/healthz`.
The real systemd backend has not been proved on live systemd — that requires
`scripts/live-check.sh`, which a human runs and does not exist yet.
