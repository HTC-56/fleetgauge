# Roadmap — the v1 scoreboard

One row per SPEC.md feature. The planning lane keeps status current; row edits here
are the one permitted exception to append-only docs.

| # | Feature (SPEC.md) | Status | Phase | Note |
|---|---|---|---|---|
| 1 | YAML fleet config | NOT BUILT | A | loader + commented example are §A7–§A9 |
| 2 | systemd backend behind an interface | PARTIAL | A | interface + systemd backend + parser round-trip shipped; fake backend is §A4–§A6 |
| 3 | Poller + ring-buffer history | NOT BUILT | — | |
| 4 | The page (self-contained, SSE) | NOT BUILT | — | hero screenshot |
| 5 | /metrics (Prometheus text) | NOT BUILT | — | |
| 6 | Triple-gated restart + action ledger | NOT BUILT | — | only mutating verb |
| 7 | /healthz + /events + structured logs | NOT BUILT | — | |
| 8 | Demo mode / fake backend | NOT BUILT | A | fake backend is §A4–§A6; `-demo` serving waits on the HTTP surface |
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
