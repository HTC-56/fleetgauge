# Decisions

## Locked (2026-08-27, at scaffold)

- **SPEC.md is the whole product.** v1 is the nine features there, fenced by
  its non-goals. The planning lane derives phases from SPEC.md only; it never
  invents features. When every SPEC.md feature is built and gated,
  "PROJECT SPEC COMPLETE" is the desired terminal state — declare it, do not
  find more work. This project is meant to FINISH.
- **Stack**: Go 1.27, stdlib + `gopkg.in/yaml.v3` — the entire dependency
  list. Single static binary, `CGO_ENABLED=0`. The page is one hand-written
  self-contained HTML file — no framework, no build step, no external
  requests.
- **The backend seam is pre-registered**: systemd is reached only through a
  narrow Go interface via subprocess (`systemctl show`, `journalctl`);
  `exec.Command` outside the systemd backend package is a spec bug. The fake
  backend is the CI engine and the demo mode; `scripts/live-check.sh` (not
  CI) is the real-systemd proof.
- **Read-only by default**: the triple-gated restart (token + per-unit
  opt-in + ledger append) is the only mutating verb in the product.
- **First Go repo for the executor**: Phase A proves the toolchain and the
  backend-interface round-trip before any HTTP surface. Structural failure
  at the stack is recorded here and the lane stands down; PROCESS.md reports
  it honestly.
- **Gates**: gofmt -l empty, go vet, go test, scrub-check — all green at
  every phase end. `verify.sh` composes them plus the README-quickstart
  lint.
- **Public-repo discipline from commit 1**: this repo will be published. No
  private hostnames, no real LAN IPs (docs use `localhost` / `192.0.2.x`),
  no absolute home paths, no key material, no references to other private
  projects — in files AND commit messages. The public HTC-56 repos may be
  named. `scrub-check.sh` enforces the file half; sessions carry the
  commit-message half.
- **Neutral git identity** until the publish decision (human-gated).

## Human-gated (never resolved by the loop)

- Publishing: flipping the repo public, name confirmation, license choice
  (default intent: MIT).
- Any claim that the real systemd backend ran — live-check.sh is run by a
  human.
- Any scope beyond SPEC.md v1.

## Open Questions

*(none — SPEC.md answers v1 in full)*
