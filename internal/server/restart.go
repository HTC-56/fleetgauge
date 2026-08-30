package server

import (
	"crypto/subtle"
	"encoding/json"
	"net"
	"net/http"
	"strings"

	"github.com/HTC-56/fleetgauge/internal/ledger"
)

// RestartJSON is the POST /units/{name}/restart response body.
//
// Field names are snake_case to match the rest of fleetgauge's JSON surface.
type RestartJSON struct {
	Name   string `json:"name"`
	Result string `json:"result"`
	Error  string `json:"error,omitempty"`
}

// HandleRestart is the only mutating handler in fleetgauge (SPEC.md feature 6).
//
// Three gates, in this order, and the order is the design:
//
//  1. The bearer token must match. No configured token — or no ledger, or no
//     backend — means the verb is off and the answer is 503, never "allowed".
//  2. The unit must carry allow_restart in the config. A valid token does not
//     grant restart of a unit the operator did not opt in.
//  3. The action is appended to the ledger BEFORE the backend is touched. If
//     that append fails the restart does not happen: an action fleetgauge
//     cannot record is an action it does not take.
//
// A second ledger line records the outcome. A failure to write that one is
// logged but does not change the response — the restart really did run, and
// claiming otherwise would be the worse lie.
func (s *Server) HandleRestart(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		respondError(w, http.StatusBadRequest, "unit name is required")
		return
	}

	if !s.RestartEnabled() {
		respondError(w, http.StatusServiceUnavailable,
			"restart is not configured: it needs a bearer token, an action ledger and a backend")
		return
	}

	// Gate 1: the token.
	if !s.authorized(r) {
		w.Header().Set("WWW-Authenticate", `Bearer realm="fleetgauge"`)
		respondError(w, http.StatusUnauthorized, "a valid bearer token is required")
		return
	}

	// Gate 2: the per-unit opt-in.
	if !s.AllowRestart(name) {
		respondError(w, http.StatusForbidden, "unit has not opted in to restart (allow_restart)")
		return
	}

	// Gate 3: write it down first.
	actor := actorOf(r)
	if err := s.ledger.Append(ledger.Entry{
		At:     s.Now(),
		Action: ledger.ActionRestart,
		Unit:   name,
		Actor:  actor,
		Result: ledger.ResultRequested,
	}); err != nil {
		s.log.Error("restart refused: ledger append failed", "unit", name, "actor", actor, "err", err)
		respondError(w, http.StatusInternalServerError, "the action could not be recorded, so it was not performed")
		return
	}

	err := s.be.Restart(r.Context(), name)

	outcome := ledger.Entry{
		At:     s.Now(),
		Action: ledger.ActionRestart,
		Unit:   name,
		Actor:  actor,
		Result: ledger.ResultOK,
	}
	if err != nil {
		outcome.Result = ledger.ResultError
		outcome.Error = err.Error()
	}
	if aerr := s.ledger.Append(outcome); aerr != nil {
		s.log.Error("restart outcome not recorded", "unit", name, "err", aerr)
	}

	if err != nil {
		s.log.Error("restart failed", "unit", name, "actor", actor, "err", err)
		respondJSON(w, http.StatusBadGateway, RestartJSON{
			Name: name, Result: ledger.ResultError, Error: err.Error(),
		})
		return
	}

	s.log.Info("restart", "unit", name, "actor", actor)
	respondJSON(w, http.StatusOK, RestartJSON{Name: name, Result: ledger.ResultOK})
}

// authorized reports whether the request carries the configured bearer token.
//
// The comparison is constant-time: the token is the only credential in the
// product, and a byte-at-a-time compare over a loopback listener is a real
// oracle, not a theoretical one.
func (s *Server) authorized(r *http.Request) bool {
	const prefix = "Bearer "

	h := r.Header.Get("Authorization")
	if len(h) <= len(prefix) || !strings.EqualFold(h[:len(prefix)], prefix) {
		return false
	}
	got := strings.TrimSpace(h[len(prefix):])

	return subtle.ConstantTimeCompare([]byte(got), []byte(s.bearerToken)) == 1
}

// actorOf names who asked, for the ledger. The host part of RemoteAddr is
// enough to answer "which machine restarted this" without recording an
// ephemeral port that means nothing an hour later.
func actorOf(r *http.Request) string {
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

// respondJSON writes v as a JSON body with the given status. It is the
// success-path twin of respondError in journal.go.
func respondJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
