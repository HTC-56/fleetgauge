package server

import (
	"context"
	"io"
	"net/http"
	"strings"
	"time"

	"fleetgauge/internal/poller"
)

// HandleEvents streams the fleet to one browser over Server-Sent Events.
//
// Two event names are emitted: "fleet", a full FleetJSON payload, and
// "transition", a single TransitionJSON. The full payload is sent immediately
// on connect so a fresh tab paints without waiting a whole broadcast interval,
// and again on every broadcast tick — the page is small enough that pushing it
// whole is simpler and more robust than diffing, and it means a browser that
// missed an event self-heals on the next one.
//
// The handler returns when the request context ends, when the hub closes, or
// on the first write error.
func (s *Server) HandleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	// Tell nginx not to buffer the stream; without it a reverse proxy holds
	// events until its buffer fills, which looks exactly like a frozen page.
	h.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	ch, cancel := s.hub.Subscribe()
	defer cancel()

	if !writeEvent(w, flusher, s.fleetEvent()) {
		return
	}

	ping := time.NewTicker(s.heartbeat)
	defer ping.Stop()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case ev, open := <-ch:
			if !open {
				return
			}
			if !writeEvent(w, flusher, ev) {
				return
			}
		case <-ping.C:
			// A comment line keeps proxies and load balancers from reaping an
			// idle connection. Clients ignore it.
			if _, err := io.WriteString(w, ": ping\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

// writeEvent writes one SSE frame and flushes it, reporting whether the write
// succeeded. A false result means the client is gone and the caller should
// stop.
func writeEvent(w io.Writer, flusher http.Flusher, ev Event) bool {
	var b strings.Builder
	if ev.Name != "" {
		b.WriteString("event: ")
		b.WriteString(ev.Name)
		b.WriteString("\n")
	}
	// SSE frames the payload one "data:" line per newline. JSON from
	// encoding/json never contains a raw newline, but splitting is what makes
	// the framing correct for any payload rather than only for that one.
	for _, line := range strings.Split(ev.Data, "\n") {
		b.WriteString("data: ")
		b.WriteString(line)
		b.WriteString("\n")
	}
	b.WriteString("\n")

	if _, err := io.WriteString(w, b.String()); err != nil {
		return false
	}
	flusher.Flush()
	return true
}

// Broadcast publishes to every SSE subscriber on a ticker until ctx is done,
// returning ctx.Err(). Each tick emits one event per transition recorded since
// the previous tick, then the current fleet.
//
// It reads only the Store, so it is decoupled from the poller entirely: the
// poller writes history at its own interval and the broadcast loop reports
// whatever it finds there. Nothing wedges if the two rates disagree.
func (s *Server) Broadcast(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	// Establish the transition watermark before the first tick so history
	// recorded before serving started is not replayed as breaking news.
	s.markTransitions()

	t := time.NewTicker(s.interval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
			s.PublishOnce()
		}
	}
}

// PublishOnce emits the transitions recorded since the last call, then the
// current fleet. Broadcast calls it on a ticker; tests call it directly.
func (s *Server) PublishOnce() {
	for _, tr := range s.newTransitions() {
		s.hub.Publish(Event{Name: "transition", Data: mustJSON(TransitionJSON{
			Unit: tr.Unit,
			From: string(tr.From),
			To:   string(tr.To),
			At:   tr.At.UTC().Format(time.RFC3339),
		})})
	}
	s.hub.Publish(s.fleetEvent())
}

// fleetEvent builds the full-payload event.
func (s *Server) fleetEvent() Event {
	return Event{Name: "fleet", Data: mustJSON(s.Fleet())}
}

// markTransitions moves the watermark to the newest recorded transition
// without emitting anything.
func (s *Server) markTransitions() {
	all := s.store.Transitions()

	s.mu.Lock()
	defer s.mu.Unlock()

	s.watermarkSet = true
	if n := len(all); n > 0 {
		s.lastTr = all[n-1]
	}
}

// newTransitions returns the transitions recorded since the previous call and
// advances the watermark.
//
// The watermark is the last transition value published, not an index: the
// store's transition log is capped and drops from the front, so an index would
// silently stop matching once it wrapped. When the remembered transition is no
// longer in the log — it aged out between calls — everything currently held is
// treated as new, which over-reports rather than losing an event.
func (s *Server) newTransitions() []poller.Transition {
	all := s.store.Transitions()

	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.watermarkSet {
		s.watermarkSet = true
		if n := len(all); n > 0 {
			s.lastTr = all[n-1]
		}
		return nil
	}

	idx := -1
	for i := len(all) - 1; i >= 0; i-- {
		if all[i] == s.lastTr {
			idx = i
			break
		}
	}

	var fresh []poller.Transition
	if idx >= 0 {
		fresh = append(fresh, all[idx+1:]...)
	} else {
		fresh = append(fresh, all...)
	}

	if n := len(all); n > 0 {
		s.lastTr = all[n-1]
	}
	return fresh
}
