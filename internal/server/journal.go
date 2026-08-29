package server

import (
	"encoding/json"
	"net/http"
)

// JournalJSON is the response shape the journal drawer reads.
//
// Field names are snake_case to match the rest of fleetgauge's JSON surface.
// Lines is always a slice (never null) so the page's .join() call never
// panics.
type JournalJSON struct {
	Name  string   `json:"name"`
	Lines []string `json:"lines"`
}

// HandleJournal serves the per-unit journal drawer.
//
// GET /units/{name}/journal — returns up to s.JournalLines() recent lines
// as a JSON array. Three error paths:
//
//   - empty name → 400
//   - no backend (nil) → 503
//   - backend returns error → 502
//
// Success is 200 with Content-Type application/json.
func (s *Server) HandleJournal(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		respondError(w, http.StatusBadRequest, "unit name is required")
		return
	}

	if s.Backend() == nil {
		respondError(w, http.StatusServiceUnavailable, "journal backend unavailable")
		return
	}

	lines, err := s.Backend().JournalTail(r.Context(), name, s.JournalLines())
	if err != nil {
		respondError(w, http.StatusBadGateway, err.Error())
		return
	}

	if lines == nil {
		lines = []string{}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(JournalJSON{Name: name, Lines: lines})
}

// respondError writes a JSON body with an "error" field and the given status.
//
// It always sets Content-Type application/json so the page can parse it.
func respondError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
