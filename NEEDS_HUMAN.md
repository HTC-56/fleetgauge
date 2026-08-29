# PROJECT SPEC COMPLETE

The loop has finished every piece of work SPEC.md authorizes. This is the
terminal state, not a stall: DECISIONS.md says v1 is the nine SPEC.md features
and that declaring completion beats inventing scope, so the planning lane is
standing down rather than opening a Phase F.

**Shipped:** Phase A (backend seam + fleet config), B (poller, history,
metrics text), C (HTTP surface, page, SSE), D (triple-gated restart + ledger),
E (deploy-grade packaging, README, CI, `docs/PROCESS.md`). All nine ROADMAP.md
rows read SHIPPED; TODO.md has no unchecked box; DECISIONS.md "Open Questions"
is empty. See ROADMAP.md for per-feature coverage — it is current.

**Verified this session, not just read off the scoreboard:** `bash verify.sh`
green (5/5 — gofmt, vet, test, scrub-check, README lint), and a real
`fleetgauge -demo` run served `/` (16 KB, zero external references), `/metrics`,
`/events` (live SSE fleet payload), `/healthz` 200, `/units/{name}/journal` 200,
and refused an untokened restart with 503.

## Decisions needed to go further

Nothing below is loop work — DECISIONS.md reserves all of it for a human.

1. **Publish the repo** — flip it public, confirm the name, choose the license
   (recorded default intent: MIT).
   *Unlocks:* the module path. It is the bare `fleetgauge` today because a
   canonical import path would presume an owner and repo name; renaming is part
   of this decision. Also settles the neutral-git-identity hold.

2. **Run `scripts/live-check.sh` on a real systemd box.**
   *Unlocks:* the one claim the docs are currently forbidden to make. The
   systemd backend has never touched live systemd — CI runs the fake backend
   only. Until a human runs it, README, STATUS.md and PROCESS.md must keep
   saying the backend is unproved.

3. **Produce the README hero screenshot** (SPEC feature 4, packaging row 9).
   *Unlocks:* the last SPEC deliverable the loop physically could not make — it
   needs a browser and a pair of eyes. `make demo`, then screenshot
   `http://localhost:8080/`.

4. **Any scope beyond SPEC.md v1** must be locked into DECISIONS.md before the
   loop restarts. With the queue empty, the loop has nothing to pick up.

## One phrasing call to make at publish time

`docs/PROCESS.md` line 186 says live-check.sh "exists and passes locally". The
next clause corrects it ("no human has run it on a real systemd system"), so it
is not a false claim, but "passes locally" reads close to the gated claim in
item 2. Docs here are append-only, so the loop left it alone. Worth a reword
before the repo goes public.
