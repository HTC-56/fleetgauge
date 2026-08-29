# Roadmap — the v1 scoreboard

One row per SPEC.md feature. The planning lane keeps status current; row edits here
are the one permitted exception to append-only docs.

| # | Feature (SPEC.md) | Status | Phase | Note |
|---|---|---|---|---|
| 1 | YAML fleet config | SHIPPED | A | loader + commented example §A7–§A9 |
| 2 | systemd backend behind an interface | SHIPPED | A | interface + systemd backend + parser round-trip + fake backend §A1–§A6 |
| 3 | Poller + ring-buffer history | SHIPPED | B | poller + ring + store + transition detection + overview + demo wiring §B1–§B10 |
| 4 | The page (self-contained, SSE) | SHIPPED | C | page + SSE hub + fleet payload + `/events` + journal + request logs §C1–§C9 |
| 5 | /metrics (Prometheus text) | SHIPPED | B, C | renderer §B8–§B9; route served from poller store §C5 |
| 6 | Triple-gated restart + action ledger | PARTIAL | D | ledger + handler + page control §D1–§D3; tests, config key and wiring are §D4–§D8 |
| 7 | /healthz + /events + structured logs | SHIPPED | C | `/events` broadcast loop §C2; `/healthz` + request logs §C4–§C5; journal §C7 |
| 8 | Demo mode / fake backend | SHIPPED | A, C | fake backend §A4–§A6; `-demo` serves the page §C9 |
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
- **The `-demo` text table is interim scaffolding** (Phase B, §B10). It polls
  the fake fleet and prints a `text/tabwriter` overview as a stopgap until the
  HTTP phase replaces it with the real page — not a claim that demo mode is
  done.
- **The hero screenshot for row 4 (the page) is a README artifact** and belongs
  to the packaging row (9), not to the page itself. The page is self-contained
  HTML; a screenshot is a packaging deliverable.
- **`allow_restart` already rides in the fleet JSON payload** while the restart
  verb does not exist yet, so the page shows no restart control until the
  restart phase lands. The flag is a placeholder for the triple-gated restart.
  *(Resolved in Phase D: the page now draws a restart button for units the
  flag opts in, and nothing for the rest.)*
- **`ledger_path` is a config key Phase D added** (§D6). SPEC feature 1 lists
  the config's keys and does not name one, but feature 6 requires a JSONL
  ledger, and a hard-coded relative path is undeployable under systemd, where
  the working directory is not the operator's. The key is plumbing feature 6
  needs, not new scope.
- **Demo mode is read-only unless `-token` is passed** (§D8). Without it there
  is no bearer token, so `RestartEnabled()` is false and every restart is a
  503 — an unattended demo cannot mutate anything. The restart verb is
  exercised by the test suite against the fake backend, and on real systemd
  only by `live-check.sh`, which a human runs.
