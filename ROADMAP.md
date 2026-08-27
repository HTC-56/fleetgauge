# Roadmap — the v1 scoreboard

One row per SPEC.md feature. The planning lane keeps status current; row edits here
are the one permitted exception to append-only docs.

| # | Feature (SPEC.md) | Status | Phase | Note |
|---|---|---|---|---|
| 1 | YAML fleet config | NOT BUILT | — | |
| 2 | systemd backend behind an interface | NOT BUILT | — | subprocess only, no cgo/D-Bus |
| 3 | Poller + ring-buffer history | NOT BUILT | — | |
| 4 | The page (self-contained, SSE) | NOT BUILT | — | hero screenshot |
| 5 | /metrics (Prometheus text) | NOT BUILT | — | |
| 6 | Triple-gated restart + action ledger | NOT BUILT | — | only mutating verb |
| 7 | /healthz + /events + structured logs | NOT BUILT | — | |
| 8 | Demo mode / fake backend | NOT BUILT | — | CI + quickstart engine |
| 9 | Deploy-grade packaging (static binary, Makefile, dual-arch CI, unit file, quickstart) | NOT BUILT | — | live-check.sh not in CI |
| — | docs/PROCESS.md (the loop story) | NOT BUILT | — | written near the end, when there is a ledger to excerpt |

When every row reads SHIPPED and verify.sh is green, the project is done — the
planning lane declares PROJECT SPEC COMPLETE rather than inventing scope.

## Reservations ledger — small deferred calls recorded inside phase specs

*(empty at scaffold; each entry names its home)*
