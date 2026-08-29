# Phase C — the HTTP surface: the page, /events, /healthz, /metrics

**Subject: ROADMAP row 4, "The page (self-contained, SSE)" (NOT BUILT).** This
phase carries row 4 to SHIPPED and takes rows 5 (`/metrics`), 7 (`/healthz` +
`/events` + structured logs) and 8 (demo mode) with it — all four were waiting
on the same thing, an HTTP surface. Nothing here touches the restart verb
(row 6); that is a later phase.

**Already shipped by the planning lane** (commits `feat(C1)`–`feat(C3)`) — do
not rewrite these, build against them:

- `internal/server/hub.go` — `Event`, `Hub`, `NewHub`, `Subscribe`, `Publish`,
  `Count`, `Dropped`, `Close`.
- `internal/server/fleet.go` — `Options`, `Server`, `New`, the accessors
  `Store` / `Hub` / `Log` / `Backend` / `JournalLines` / `Now` / `Close`, the
  JSON types (`UnitJSON`, `TransitionJSON`, `CountsJSON`, `FleetJSON`), and
  `Fleet()` which builds the whole payload.
- `internal/server/sse.go` — `HandleEvents` (the `/events` handler),
  `Broadcast(ctx)`, `PublishOnce()`.
- `internal/server/doc.go` — the package comment. **Do not write a second
  package comment in any other file in this package.**
- `internal/server/server_smoke_test.go` — the thin end-to-end test. **Your
  pattern file for every test task below.** It defines `pinnedClock()`,
  `newTestServer(t, polls)` and `waitFor(t, what, cond)` — reuse them, never
  redefine them; a second definition in the same package is a compile error.
- `internal/page/` — `index.html` (the page) and `page.go` exposing
  `page.HTML() []byte`, `page.ContentType` and `page.ContentLength()`.

**The gate for every task below** (all four, every time; red = not done):

```
gofmt -l .            # must print nothing
go vet ./...
go test ./...
bash scripts/scrub-check.sh
```

---

## §C4 — Structured request logging middleware

Create `internal/server/log.go` and `internal/server/log_test.go`. Package
`server`.

SPEC.md feature 7 asks for structured logs to stderr. `log/slog` is stdlib;
add no dependency.

Write one exported function: `LogRequests(log *slog.Logger, next http.Handler)
http.Handler`. It serves the request through `next`, then emits exactly one
`Info` record per request with these attributes: `method`, `path`, `status`,
and `duration_ms` (a float or int of your choosing, but name it exactly that).

To know the status you need a small unexported wrapper type around
`http.ResponseWriter` that remembers what was passed to `WriteHeader`, and
defaults to `200` when a handler writes a body without calling `WriteHeader`.

**The one trap in this task:** that wrapper must also implement
`http.Flusher` — a `Flush()` method that forwards to the wrapped writer when
the wrapped writer supports it. `HandleEvents` type-asserts its
`ResponseWriter` to `http.Flusher` and returns a 500 if the assertion fails,
so a wrapper without `Flush()` silently breaks `/events` and the page never
paints.

Do not write a package comment; `doc.go` already has it.

Tests in `log_test.go`, mirroring the plain-`testing` style of
`server_smoke_test.go`. Build the logger with
`slog.New(slog.NewJSONHandler(&buf, nil))` so the record is parseable, and
drive it with `httptest.NewRecorder()`.

Assert, in prose:

1. A handler that returns normally produces exactly one log record, and that
   record's `method` and `path` match the request.
2. A handler that calls `WriteHeader(404)` logs `status` 404.
3. A handler that writes a body without calling `WriteHeader` logs `status`
   200.
4. The wrapper satisfies `http.Flusher`: inside the wrapped handler, a type
   assertion of the `ResponseWriter` to `http.Flusher` succeeds, and calling
   `Flush()` does not panic.
5. The response body the client receives is unchanged by the wrapping.

Gate: the four commands above.

---

## §C5 — server.go: the mux and the three thin handlers

Create `internal/server/server.go`. Package `server`. No package comment.

This is the routing table plus three handlers that are each a few lines. It is
the last piece needed before the binary can serve anything.

Write:

1. `func (s *Server) Handler() http.Handler` — builds an `*http.ServeMux`,
   registers the four routes below, and returns the mux wrapped in
   `LogRequests(s.Log(), mux)` from §C4.
2. `func (s *Server) HandleIndex(w http.ResponseWriter, r *http.Request)` —
   writes `page.HTML()` with `Content-Type: page.ContentType`. Import
   `fleetgauge/internal/page`.
3. `func (s *Server) HandleHealthz(w http.ResponseWriter, r *http.Request)` —
   see the status rule below.
4. `func (s *Server) HandleMetrics(w http.ResponseWriter, r *http.Request)` —
   writes `metrics.Render(s.Store(), s.Now())` with Content-Type
   `text/plain; version=0.0.4; charset=utf-8`. Import
   `fleetgauge/internal/metrics`.

The four routes, using Go's method-and-pattern syntax:

```
GET /{$}
GET /healthz
GET /metrics
GET /events
```

`/events` maps to the existing `s.HandleEvents` — do not write a new one. The
`{$}` matters: without it the page pattern also swallows every unknown path,
and a typo would render the dashboard instead of a 404.

**The healthz rule**, stated plainly: read `s.Store().LastPoll()`. If it
returns a non-nil error, or a zero time (nothing has ever been polled), the
service is not healthy — respond `503` with a JSON body whose `status` field
is `"degraded"`. Otherwise respond `200` with `status` `"ok"`. Either way the
body is JSON with Content-Type `application/json` and also carries `units`
(the length of `s.Store().Names()`), `polls` and `failures` (both from
`s.Store().Counts()`), and `snapshot_age_seconds` (from
`s.Store().SnapshotAge(s.Now())`). Declare a small exported `HealthJSON`
struct for it, tagged the same snake_case way as `FleetJSON` in `fleet.go` —
that file is your pattern for struct tags and doc-comment style.

Gate: the four commands above.

---

## §C6 — server_test.go: the routing and handler suite

Create `internal/server/server_test.go`. Package `server`.

Mirror `server_smoke_test.go` and use its `newTestServer(t, polls)` helper.
Drive requests through `srv.Handler()` with `httptest.NewRecorder()` and
`httptest.NewRequest`.

Assert, in prose:

1. `GET /` returns 200, a `Content-Type` starting with `text/html`, and a body
   containing `<!DOCTYPE html>` and the string `EventSource`.
2. `GET /metrics` returns 200 with a `text/plain` Content-Type, and its body
   contains `fleetgauge_unit_up{unit="nginx.service"} 1`.
3. `GET /healthz` on a server whose store has been polled returns 200 and a
   JSON body whose `status` is `"ok"` and whose `units` is 12.
4. `GET /healthz` on a server built over a bare `poller.NewStore(5)` that has
   never been polled returns 503 and `status` `"degraded"`. Build that one with
   `New(Options{Store: poller.NewStore(5)})` directly rather than through
   `newTestServer`.
5. `GET /nope` returns 404 — the page pattern does not swallow unknown paths.
6. `POST /` returns 405: the read-only surface accepts only GET.

Gate: the four commands above.

---

## §C7 — The journal drawer endpoint

Create `internal/server/journal.go` and `internal/server/journal_test.go`, and
add one route line to `Handler()` in `server.go`.

The page's per-unit drawer (SPEC feature 4) fetches
`/units/<name>/journal` and expects JSON with a `lines` array of strings. It
already degrades to "journal unavailable" on any non-200, so a clean error
response is a real answer, not a failure.

Write `func (s *Server) HandleJournal(w http.ResponseWriter, r *http.Request)`
and register it in `Handler()` as `GET /units/{name}/journal`. Read the unit
name with `r.PathValue("name")`.

Behaviour:

1. Empty name → `400`.
2. `s.Backend()` is nil → `503`. It is a legal configuration (the field is
   optional), so this must not panic.
3. Otherwise call `s.Backend().JournalTail(r.Context(), name, s.JournalLines())`.
   A non-nil error → `502`.
4. Success → `200`, Content-Type `application/json`, body carrying the unit
   name and the lines. Declare a small exported `JournalJSON` struct with
   snake_case tags; `fleet.go` is your pattern. A nil slice of lines must
   marshal as `[]`, not `null` — the page's JavaScript does `.join` on it.

Error responses are JSON too, with an `error` field.

Tests in `journal_test.go`, using `newTestServer(t, 1)` — the fake backend
returns synthetic lines for any unit name.

Assert, in prose:

1. `GET /units/nginx.service/journal` returns 200 and a JSON body whose
   `lines` array is non-empty.
2. The number of lines returned does not exceed `srv.JournalLines()`.
3. A server built with `New(Options{Store: ...})` and no Backend returns 503
   for the same path, and does not panic.
4. The response Content-Type is `application/json`.

Gate: the four commands above.

---

## §C8 — hub_test.go: the fan-out and stream suite

Create `internal/server/hub_test.go`. Package `server`.

Mirror `server_smoke_test.go` and reuse its `newTestServer` and `waitFor`
helpers. `waitFor` is how you synchronise on a goroutine — never a bare
`time.Sleep`.

Assert, in prose:

1. Two subscribers both receive the same published event, and `Count()`
   reports 2 while they are live and 0 after both cancels run.
2. Calling a subscriber's cancel twice does not panic and does not double-close
   its channel.
3. A subscriber that never reads stops receiving once its buffer fills:
   publish more than `subscriberBuffer` events to it and `Dropped()` becomes
   greater than zero, while `Publish` itself still returns promptly.
4. After `Close()`, `Count()` is 0, a further `Publish` is a no-op, and a new
   `Subscribe()` returns a channel that is already closed (a receive yields
   `ok == false` immediately).
5. `HandleEvents` sets `Cache-Control: no-cache` and, when the request's
   context is cancelled, returns and leaves `Hub().Count()` at 0. Follow the
   goroutine-plus-`waitFor`-plus-cancel shape in `TestSmokeEventsStreamsTheFleet`.
6. After a `be.Tick()` and another `PollOnce`, calling `srv.PublishOnce()`
   makes a live subscriber receive at least one event whose `Name` is
   `"transition"`, and its `Data` mentions `flappy.service`.

Gate: the four commands above.

---

## §C9 — Serve for real: wire main.go to the HTTP surface

Edit `cmd/fleetgauge/main.go`. This replaces the interim `-demo` text table
that Phase B recorded as scaffolding in the ROADMAP reservations ledger — that
replacement is pre-authorised, it is what this phase is for.

Both modes now serve HTTP instead of printing a table and exiting.

1. `runDemo`: build `fake.New()`, a poller over it with pattern `*.service`,
   a 1-second interval and depth 120. Listen address: `-addr` if given, else
   `127.0.0.1:8080`.
2. `runReal`: keep the existing `config.Load` and `systemd.New()` lines and the
   pattern extraction exactly as they are. Listen address: `-addr` if given,
   else `cfg.Listen`. Build an `allowRestart` map from `cfg.Units`
   (`u.Name` → `u.AllowRestart`) and pass `cfg.JournalLines` as
   `Options.JournalLines`.
3. Both then do the same five things, so factor them into one helper that takes
   the backend, the poller, the address and the `server.Options` extras:
   - build the server with `server.New(...)`, `defer srv.Close()`;
   - `ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)`,
     `defer stop()`;
   - `go p.Run(ctx)` and `go srv.Broadcast(ctx)`;
   - an `http.Server` with the address and `srv.Handler()`, started in a
     goroutine with `ListenAndServe`; log the listen address through
     `srv.Log()` first so the operator sees where to point a browser;
   - block on `<-ctx.Done()`, then `hs.Shutdown(context.Background())`.
     `ListenAndServe` returning `http.ErrServerClosed` is the success path,
     not an error — check for it with `errors.Is`.

Remove `printOverview`, `formatMemory`, `formatUptime` and the now-unused
`text/tabwriter` import together; nothing else calls them, and an unused import
does not compile. Update the file's doc comment, which still says there is no
HTTP surface yet.

Stdlib only. No `exec.Command` in this file — `scripts/scrub-check.sh` fails
the task if it appears outside `internal/backend/systemd/`.

Gate: the four commands above, plus `go run ./cmd/fleetgauge -demo` printing a
listen line and serving; `curl -s localhost:8080/healthz` returns JSON with
`"status":"ok"` and `curl -s localhost:8080/metrics` returns the metric text.
Stop it with Ctrl-C — or run it with a timeout so the session does not hang on
a foreground server.

---

## §C10 — verify.sh, then STATUS.md and ROADMAP.md

No new code. Run the gates, then update the two docs.

1. Run `bash verify.sh`. It must print `verify: all clear` and exit 0. If it
   does not, fix what it reports before touching the docs.
2. **STATUS.md** — append a `## Phase C` section in the shape of the existing
   `## Phase B` section. Say what shipped: the SSE hub and broadcast loop, the
   self-contained page, `/events`, `/healthz`, the served `/metrics`, the
   journal drawer endpoint, structured request logs, and `-demo` now serving
   the page instead of printing a table. Say plainly what is still missing: the
   triple-gated restart and its action ledger are not built, so the product is
   still entirely read-only; there is no README, no Makefile and no CI; and the
   real systemd backend is still unproved on live systemd, because
   `scripts/live-check.sh` does not exist and a human runs it.
3. **ROADMAP.md** — flip rows 4, 5, 7 and 8 to `SHIPPED`, phase `C`, each with
   a one-line note. Leave rows 6 and 9 and the PROCESS.md row alone.
4. In the ROADMAP reservations ledger, record two things: that the hero
   screenshot for row 4 is a README artifact and belongs to the packaging row
   (9), not to the page itself; and that `allow_restart` already rides in the
   fleet JSON payload while the restart verb does not exist yet, so the page
   shows no restart control until the restart phase lands.

Gate: `bash verify.sh` green, plus the four commands above.
