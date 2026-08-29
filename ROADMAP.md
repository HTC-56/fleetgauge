# Roadmap — the v1 scoreboard

One row per SPEC.md feature. The planning lane keeps status current; row edits here
are the one permitted exception to append-only docs.

| # | Feature (SPEC.md) | Status | Phase | Note |
|---|---|---|---|---|
| 1 | YAML fleet config | SHIPPED | A | loader + commented example §A7–§A9 |
| 2 | systemd backend behind an interface | SHIPPED | A | interface + systemd backend + parser round-trip + fake backend §A1–§A6 |
| 3 | Poller + ring-buffer history | PARTIAL | B | poller + ring + store + transition detection shipped §B1–§B3; test suites, overview and demo wiring in flight |
| 4 | The page (self-contained, SSE) | NOT BUILT | — | hero screenshot |
| 5 | /metrics (Prometheus text) | NOT BUILT | — | renderer authored in Phase B §B8; the endpoint needs the HTTP surface |
| 6 | Triple-gated restart + action ledger | NOT BUILT | — | only mutating verb |
| 7 | /healthz + /events + structured logs | NOT BUILT | — | |
| 8 | Demo mode / fake backend | PARTIAL | A | fake backend §A4–§A6 exists and is the test engine; `-demo` cannot serve until the HTTP surface lands |
| 9 | Deploy-grade packaging (static binary, Makefile, dual-arch CI, unit file, quickstart) | NOT BUILT | — | live-check.sh not in CI |
| — | docs/PROCESS.md (the loop story) | NOT BUILT | — | written near the end, when there is a ledger to excerpt |

When every row reads SHIPPED and verify.sh is green, the project is done — the
planning lane declares PROJECT SPEC COMPLETE rather than inventing scope.

## Reservations ledger — small deferred calls recorded inside phase specs

- **Module path is the bare `fleetgauge`** (Phase A, `feat(A1)`). A hosting URL
  would presume a repo name and owner, and both are human-gated in
  DECISIONS.md. Renaming to a canonical import path is part of the publish
  decision, not loop work.
- **`scripts/live-check.sh` is unwritten** (Phase A). Until it exists and a
  human runs it, no claim that the real systemd backend ran may appear in the
  README, STATUS.md, or PROCESS.md. Its home is the packaging row (9).
