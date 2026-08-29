# fleetgauge

One static Go binary that watches a declared fleet of systemd services and
serves one page answering "is everything up?" at a glance — plus Prometheus
metrics, a live SSE feed, and a token-gated restart for the units that opt in.
Ops sensibility as a work sample: restart counts, memory, state transitions,
journal tails, and an interface seam that makes systemd itself swappable in
tests.

## What you need

Go 1.27+. systemd is **not** required for the demo or the test suite — the
fake backend covers every code path.

## Demo in one minute

```
go run ./cmd/fleetgauge -demo
```

Then open `http://127.0.0.1:8080`. You'll see a synthetic twelve-unit fleet
with states drifting in real time over SSE: one unit flapping between active
and failed, another wedged in failure, the rest steady. Use `-addr` to listen
on a different address.

Demo mode is read-only. No restart buttons render unless you pass `-token`,
and even then only units marked `allow_restart: true` in the config show them.
This is deliberate — an unattended demo must not be able to mutate anything.

## Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/` | The single-page dashboard |
| GET | `/healthz` | 503 until the first poll lands, then 200 |
| GET | `/metrics` | Prometheus text exposition |
| GET | `/events` | SSE transition stream |
| GET | `/units/{name}/journal` | Journal tail drawer |
| POST | `/units/{name}/restart` | Triple-gated restart (bearer token + opt-in + ledger) |
