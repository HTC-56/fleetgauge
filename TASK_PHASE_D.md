# Phase D — the triple-gated restart and the action ledger

**Subject: ROADMAP row 6, "Triple-gated restart + action ledger" (NOT BUILT).**
This phase carries row 6 to SHIPPED. It is the only mutating verb in the whole
product; everything else stays read-only. Row 9 (packaging: README, Makefile,
CI, unit file) is a later phase — nothing here touches it.

**Already shipped by the planning lane** (commits `feat(D1)`–`feat(D3)`) — do
not rewrite these, build against them:

- `internal/ledger/ledger.go` — `Entry`, `Ledger`, `Open`, `Append`, `Path`,
  `Close`, `ErrClosed`, and the constants `ActionRestart`, `ResultRequested`,
  `ResultOK`, `ResultError`.
- `internal/server/restart.go` — `RestartJSON` and `HandleRestart`, the three
  gates, plus the `respondJSON` helper.
- `internal/server/fleet.go` — grew the `Appender` interface and two Options
  fields, `BearerToken` and `Ledger`; `Server` grew `AllowRestart(name)` and
  `RestartEnabled()`.
- `internal/server/server.go` — the route `POST /units/{name}/restart`.
- `internal/page/index.html` — the Action column, the token box, the POST.

**The gate for every task below** (all four, every time; red = not done):

```
gofmt -l .            # must print nothing
go vet ./...
go test ./...
bash scripts/scrub-check.sh
```

---

## §D4 — ledger_test.go: the append-only file suite

Create `internal/ledger/ledger_test.go`. Package `ledger`.

Plain `testing`, no helpers to import. Write every ledger into
`filepath.Join(t.TempDir(), "actions.jsonl")` so nothing touches the repo.
Read a ledger back with `os.ReadFile` and split on `"\n"`, decoding each
non-empty line with `json.Unmarshal` into an `Entry`.

`internal/config/config_test.go` is your pattern file for the shape of a
table-free Go test that writes files into `t.TempDir()`.

Assert, in prose:

1. `Open` on a path that does not exist creates the file, and `Path()` returns
   the path it was given.
2. After two `Append` calls the file holds exactly two lines, each of which is
   one complete JSON object — decoding line 1 alone succeeds.
3. Every field survives the round trip: append an `Entry` with a non-zero
   `At`, plus `Action`, `Unit`, `Actor`, `Result` and `Error` all set, decode
   it back, and compare each field. Compare the timestamp with `.Equal`, not
   `==`.
4. Reopening an existing ledger appends rather than truncates: `Open`, append
   one, `Close`, `Open` the same path again, append one more, and the file
   holds two lines.
5. `Append` after `Close` returns `ErrClosed` (check with `errors.Is`), and a
   second `Close` returns nil rather than panicking.
6. Concurrent appends do not lose or tear a line: launch 20 goroutines that
   each append once, wait on a `sync.WaitGroup`, then assert the file has 20
   lines and every one of them decodes cleanly.

Gate: the four commands above.

---

## §D5 — restart_test.go: the three gates

Create `internal/server/restart_test.go`. Package `server`.

`internal/server/server_test.go` is your pattern file for driving requests:
build the request with `httptest.NewRequest`, serve it through
`srv.Handler()` into an `httptest.NewRecorder()`, and read `rec.Code`.

`newTestServer` does not configure restart, so this file needs two small
task-local helpers of its own:

- A stub `Appender`: a struct with a slice of `ledger.Entry`, a `fail bool`,
  and an `Append` method that returns an error when `fail` is set and
  otherwise records the entry. Guard the slice with a mutex.
- A constructor that builds a `fake.New()` backend, polls it once through a
  `poller.Poller`, and returns BOTH that backend and a `New(Options{...})`
  carrying `Store`, `Backend`, `Now: pinnedClock()`, plus the `BearerToken`,
  `Ledger` and `AllowRestart` map the caller passes in. Assertion 5 needs the
  backend, which is why it comes back too. Mark `nginx.service` as the
  opted-in unit.

Reuse `pinnedClock()` — never redefine it, and never redefine `newTestServer`
either.

Assert, in prose:

1. A server built with an empty `BearerToken` answers 503 to
   `POST /units/nginx.service/restart` even when the request carries a token.
2. With a token configured, a request with no `Authorization` header is 401,
   and so is one carrying the wrong token.
3. A correct token aimed at `redis.service` — a unit that is not opted in —
   is 403, and the stub ledger recorded nothing at all for it.
4. A correct token aimed at `nginx.service` is 200, the JSON body's `result`
   is `"ok"`, and the stub ledger holds exactly two entries whose `Result`
   values are `"requested"` then `"ok"` in that order.
5. When the stub `Appender` is set to fail, the same request is 500 and the
   fake backend's restart count for the unit did not change — the action was
   refused, not merely unrecorded. (Read the count through
   `be.Show(ctx, []string{"nginx.service"})` before and after.)
6. `GET /units/nginx.service/restart` is 405: the route is POST-only.

Gate: the four commands above.

---

## §D6 — config: where the ledger lives, and a token that must exist

Edit `internal/config/config.go` and add tests to
`internal/config/config_test.go`. Two focused changes plus their assertions.

1. Add a `LedgerPath string` field with the yaml tag `ledger_path`, alongside
   the existing `BearerToken` field. When it is empty after unmarshalling,
   default it to `"ledger.jsonl"` — put that beside the other defaults in
   `Load`, in the same style.
2. Add one validation rule to the block below the defaults: if any unit has
   `AllowRestart` true and `BearerToken` is empty, return an error. A restart
   opt-in with no token to check would be an opt-in nothing enforces. Phrase
   the error like its neighbours — it names the file and the failing key.

Do not add any other key, and do not change the existing defaults.

Add three tests, mirroring `TestConfigDefaults` and `TestConfigEmptyUnits`:

1. A config with no `ledger_path` loads with `LedgerPath == "ledger.jsonl"`.
2. A config that sets `ledger_path` keeps the value it was given.
3. A config with a unit carrying `allow_restart: true` and no `bearer_token`
   fails to load, and the error message mentions `bearer_token`.

Gate: the four commands above. `TestConfigExampleLoad` must still pass — the
shipped example already sets `bearer_token`, so it will.

---

## §D7 — the example config documents the ledger

Edit `deploy/fleetgauge.example.yaml` only. No Go changes.

1. Add a commented `ledger_path` key documenting what it is (the append-only
   JSONL file every restart attempt is written to, one JSON object per line),
   its default `ledger.jsonl`, and the fact that a relative path resolves
   against the process's working directory — so a config running under systemd
   should give an absolute path under `/var/lib`. Set it to
   `"/var/lib/fleetgauge/ledger.jsonl"` in the example.
2. Fix the `journal_lines` comment. It currently says the lines are attached to
   "each beat's visual layer", which describes nothing in this program. They
   are the lines the per-unit journal drawer on the page shows.
3. Extend the `bearer_token` comment to state the rule §D6 added: a config
   where any unit sets `allow_restart: true` must set a non-empty token, or
   loading fails.

Keep the placeholder token exactly as it is. Public-repo rule still binds:
only `localhost`, `127.0.0.1` or `192.0.2.x` may appear.

Gate: the four commands above. `TestConfigExampleLoad` proves the example
still parses — if it fails, the example drifted from the loader.

---

## §D8 — main.go: hand the server a token and a ledger

Edit `cmd/fleetgauge/main.go`. Four changes; `serve()` itself does not change
apart from what its caller puts in `server.Options`.

1. Add a `-token` flag (string, default empty), documented as "bearer token
   for demo mode; real mode reads it from the config file".
2. In `runReal`: open the ledger with `ledger.Open(cfg.LedgerPath)`, return a
   wrapped error if that fails, `defer l.Close()`, and set both
   `BearerToken: cfg.BearerToken` and `Ledger: l` on the `opts` value it
   already builds.
3. In `runDemo`: when `-token` is empty, leave everything as it is — demo mode
   stays read-only and the page draws no buttons. When `-token` is set, open a
   ledger at `"ledger.jsonl"`, `defer l.Close()`, and pass `Options` carrying
   that ledger, `BearerToken` set to the flag, and an `AllowRestart` map
   marking exactly three units true: `nginx.service`, `worker.service` and
   `flappy.service`. Three of twelve, so the page shows both the opted-in and
   the opted-out case.
4. Update the file's doc comment for the new flag.

`runDemo` currently takes one argument; give it the token too. Stdlib plus the
existing imports only — `ledger` is a new import, nothing else is.

Gate: the four commands above, plus a live check. Run
`go run ./cmd/fleetgauge -demo -token dev-token -addr 127.0.0.1:8137` with a
timeout so the session does not hang on a foreground server, and confirm:
`curl -s -X POST -H 'Authorization: Bearer dev-token'
localhost:8137/units/nginx.service/restart` returns `"result":"ok"`, the same
call with a wrong token returns 401, and the same call against
`redis.service` returns 403. `ledger.jsonl` is gitignored — never stage it.

---

## §D9 — page_test.go: the page stays self-contained

Create `internal/page/page_test.go`. Package `page`.

There is no test file in this package yet. It needs no fixtures: read the
embedded bytes with `string(HTML())` and assert on that string with
`strings.Contains` and `strings.Count`.

SPEC.md's non-goals bind this file — inline CSS and JS only, no framework, no
build step, no CDN, no web fonts, no external request of any kind — and until
now nothing enforced them. This test is that enforcement.

Assert, in prose:

1. `HTML()` is non-empty, starts with `<!DOCTYPE html>`, and `ContentLength()`
   equals the length of `HTML()` as a decimal string.
2. The page makes no external request: the string contains neither `http://`
   nor `https://`, and contains no `<link` tag and no `@import`.
3. The restart control is present: the page contains `id="token"`, the string
   `/restart`, and `"Bearer "`.
4. The restart POST is a POST: the page contains `method: "POST"`.
5. The drawer's colSpan matches the table width — the page contains
   `colSpan = 7` and the `<thead>` row has 7 `<th` occurrences. (Adding a
   column without widening the drawer leaves a visibly broken row, and no Go
   type catches it.)
6. `ContentType` is `text/html; charset=utf-8`.

Gate: the four commands above.

---

## §D10 — verify.sh, then STATUS.md and ROADMAP.md

No new code. Run the gates, then update the two docs.

1. Run `bash verify.sh`. It must print `verify: all clear` and exit 0. If it
   does not, fix what it reports before touching the docs.
2. **STATUS.md** — append a `## Phase D` section in the shape of the existing
   `## Phase C` section. Say what shipped: the append-only JSONL action
   ledger, the triple-gated `POST /units/{name}/restart` (token, per-unit
   opt-in, ledger-before-execute), the `ledger_path` config key and the rule
   that an `allow_restart` unit requires a token, the restart control on the
   page, and demo mode's `-token` flag. Say plainly what is still missing:
   there is no README, no Makefile, no CI and no example systemd unit file, so
   the packaging row is untouched; `docs/PROCESS.md` is unwritten; and the
   real systemd backend is still unproved on live systemd, because
   `scripts/live-check.sh` does not exist and a human runs it — which means no
   claim that a real restart has ever run may appear anywhere.
3. **ROADMAP.md** — flip row 6 to `SHIPPED`, phase `D`, with a one-line note.
   Leave row 9 and the PROCESS.md row alone. Update the row-4 note to mention
   the restart control now on the page.
4. In the ROADMAP reservations ledger, record two things: that `ledger_path`
   is a config key Phase D added because feature 6 needs the ledger to have a
   home a systemd unit can point at, and feature 1's key list did not name
   one; and that demo mode is read-only unless `-token` is passed, so the
   restart verb is exercised by the test suite and by `live-check.sh`, never
   by an unattended demo.

Gate: `bash verify.sh` green, plus the four commands above.
