package server

import (
	"encoding/json"
	"net/http"

	"fleetgauge/internal/metrics"
	"fleetgauge/internal/page"
)

// HealthJSON is the /healthz response body.
//
// Field names are snake_case to match the rest of fleetgauge's JSON surface.
type HealthJSON struct {
	Status         string  `json:"status"`
	Units          int     `json:"units"`
	Polls          int     `json:"polls"`
	Failures       int     `json:"failures"`
	SnapshotAgeSec float64 `json:"snapshot_age_seconds"`
}

// Handler returns the top-level HTTP handler: a ServeMux wired to the
// fleetgauge routes (/, /healthz, /metrics, /events, the journal drawer and
// the restart verb), wrapped in the LogRequests middleware so every call is
// timestamped.
//
// Every route but one is a GET. POST /units/{name}/restart is registered with
// its method spelled out, so a GET of that path is a 405 rather than a way to
// restart a service by following a link.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /{$}", s.HandleIndex)
	mux.HandleFunc("/healthz", s.HandleHealthz)
	mux.HandleFunc("/metrics", s.HandleMetrics)
	mux.HandleFunc("/events", s.HandleEvents)
	mux.HandleFunc("/units/{name}/journal", s.HandleJournal)
	mux.HandleFunc("POST /units/{name}/restart", s.HandleRestart)

	return LogRequests(s.log, mux)
}

// HandleIndex serves the embedded HTML dashboard.
func (s *Server) HandleIndex(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", page.ContentType)
	w.Header().Set("Content-Length", page.ContentLength())
	w.WriteHeader(http.StatusOK)
	w.Write(page.HTML())
}

// HandleHealthz reports whether the poller has ever completed a cycle.
//
// 200 with status "ok" when the last poll succeeded and carried a timestamp.
// 503 with status "degraded" when the store has never been polled or the last
// poll failed. The body always carries fleet-wide counts.
func (s *Server) HandleHealthz(w http.ResponseWriter, _ *http.Request) {
	now, err := s.store.LastPoll()
	degraded := err != nil || now.IsZero()

	polls, failures := s.store.Counts()
	health := HealthJSON{
		Units:          len(s.store.Names()),
		Polls:          polls,
		Failures:       failures,
		SnapshotAgeSec: s.store.SnapshotAge(s.Now()).Seconds(),
	}

	if degraded {
		health.Status = "degraded"
		w.WriteHeader(http.StatusServiceUnavailable)
	} else {
		health.Status = "ok"
		w.WriteHeader(http.StatusOK)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}

// HandleMetrics renders Prometheus exposition format from the store.
func (s *Server) HandleMetrics(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(metrics.Render(s.store, s.Now())))
}
