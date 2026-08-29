// Package server is fleetgauge's HTTP surface: the page at /, the Prometheus
// text at /metrics, the liveness probe at /healthz, the SSE stream at /events,
// and the per-unit journal drawer.
//
// Everything here reads the poller's Store, which is the single source of
// truth for unit state. No handler polls the backend for state — the journal
// drawer is the one exception, because journal lines are fetched on demand
// rather than sampled on a timer.
//
// The Store is safe for concurrent use and every accessor returns copies, so
// handlers take no locks of their own.
package server
