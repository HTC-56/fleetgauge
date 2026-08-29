package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestHubTwoSubscribersBothReceive verifies that two live subscribers both
// see every published event and that Count tracks them accurately.
func TestHubTwoSubscribersBothReceive(t *testing.T) {
	h := NewHub()
	defer h.Close()

	ch1, cancel1 := h.Subscribe()
	ch2, cancel2 := h.Subscribe()

	if got := h.Count(); got != 2 {
		t.Errorf("Count() = %d after two Subscribe, want 2", got)
	}

	h.Publish(Event{Name: "fleet", Data: `{"ok":true}`})

	select {
	case ev := <-ch1:
		if ev.Name != "fleet" {
			t.Errorf("ch1 event name = %q, want %q", ev.Name, "fleet")
		}
	case <-time.After(time.Second):
		t.Fatal("ch1: published event never arrived")
	}

	select {
	case ev := <-ch2:
		if ev.Name != "fleet" {
			t.Errorf("ch2 event name = %q, want %q", ev.Name, "fleet")
		}
	case <-time.After(time.Second):
		t.Fatal("ch2: published event never arrived")
	}

	cancel1()
	cancel2()

	if got := h.Count(); got != 0 {
		t.Errorf("Count() = %d after both cancel, want 0", got)
	}
}

// TestHubDoubleCancelDoesNotPanic verifies that calling cancel twice is safe.
func TestHubDoubleCancelDoesNotPanic(t *testing.T) {
	h := NewHub()
	defer h.Close()

	_, cancel := h.Subscribe()

	// Double cancel must not panic.
	cancel()
	cancel()

	if got := h.Count(); got != 0 {
		t.Errorf("Count() = %d after double cancel, want 0", got)
	}
}

// TestHubDropOnSlowSubscriber verifies that a subscriber that never reads
// stops receiving once its buffer fills and Dropped counts the misses.
func TestHubDropOnSlowSubscriber(t *testing.T) {
	h := NewHub()
	defer h.Close()

	ch, cancel := h.Subscribe()
	defer cancel()

	// subscriberBuffer is 16; drain the buffer first so we can overflow.
	for i := 0; i < subscriberBuffer; i++ {
		h.Publish(Event{Name: "fill", Data: "x"})
		<-ch
	}

	// Now fill the buffer and push one more — that one should be dropped.
	for i := 0; i <= subscriberBuffer; i++ {
		h.Publish(Event{Name: "overflow", Data: "x"})
	}

	// The channel should still be readable (some events landed).
	select {
	case ev := <-ch:
		if ev.Name != "overflow" {
			t.Errorf("received event name = %q, want %q", ev.Name, "overflow")
		}
	case <-time.After(time.Second):
		t.Fatal("expected to receive an event on the channel")
	}

	if got := h.Dropped(); got < 1 {
		t.Errorf("Dropped() = %d, want at least 1", got)
	}

	// Publish itself should return promptly (it never blocks).
	done := make(chan struct{})
	go func() {
		h.Publish(Event{Name: "late", Data: "x"})
		close(done)
	}()
	select {
	case <-done:
		// good — Publish returned promptly.
	case <-time.After(time.Second):
		t.Fatal("Publish blocked instead of returning promptly")
	}
}

// TestHubCloseSemantics verifies that Close zeroes Count, Publish becomes a
// no-op, and new Subscribe returns an already-closed channel.
func TestHubCloseSemantics(t *testing.T) {
	h := NewHub()

	_, cancel1 := h.Subscribe()
	defer cancel1()

	if got := h.Count(); got != 1 {
		t.Errorf("Count() = %d after one Subscribe, want 1", got)
	}

	h.Close()

	if got := h.Count(); got != 0 {
		t.Errorf("Count() = %d after Close, want 0", got)
	}

	// Further Publish is a no-op — must return promptly.
	done := make(chan struct{})
	go func() {
		h.Publish(Event{Name: "after", Data: "x"})
		close(done)
	}()
	select {
	case <-done:
		// good
	case <-time.After(time.Second):
		t.Fatal("Publish after Close blocked")
	}

	// A new subscriber gets an already-closed channel.
	ch2, cancel2 := h.Subscribe()
	defer cancel2()

	select {
	case _, open := <-ch2:
		if open {
			t.Error("received from a channel that should be closed (ok == true)")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for channel close signal")
	}
}

// TestHubHandleEventsContextCancel verifies that HandleEvents sets
// Cache-Control: no-cache, returns when the request context is cancelled,
// and leaves Hub().Count() at zero.
func TestHubHandleEventsContextCancel(t *testing.T) {
	srv, _, _ := newTestServer(t, 2)

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/events", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		defer close(done)
		srv.HandleEvents(rec, req)
	}()

	// Wait for the subscriber to register.
	waitFor(t, "the SSE subscriber to register", func() bool {
		return srv.Hub().Count() == 1
	})

	// Now cancel the request context.
	cancel()
	<-done

	if got := rec.Header().Get("Cache-Control"); got != "no-cache" {
		t.Errorf("Cache-Control = %q, want %q", got, "no-cache")
	}

	if srv.Hub().Count() != 0 {
		t.Errorf("Hub().Count() = %d after context cancel, want 0", srv.Hub().Count())
	}
}

// TestHubTransitionEvent verifies that after a Tick + PollOnce, PublishOnce
// emits a "transition" event whose Data mentions flappy.service.
func TestHubTransitionEvent(t *testing.T) {
	srv, be, p := newTestServer(t, 2)

	ch, cancel := srv.Hub().Subscribe()
	defer cancel()

	// First PublishOnce establishes the watermark (returns nil transitions).
	srv.PublishOnce()

	// Tick the backend and poll once more so the store records a new
	// transition (flappy.service flips state each Tick).
	be.Tick()
	if _, err := p.PollOnce(context.Background()); err != nil {
		t.Fatalf("PollOnce: %v", err)
	}

	// Second PublishOnce emits the new transition.
	srv.PublishOnce()

	// A live subscriber should see at least one "transition" event.
	deadline := time.Now().Add(2 * time.Second)
	found := false
	for time.Now().Before(deadline) {
		select {
		case ev := <-ch:
			if ev.Name == "transition" && strings.Contains(ev.Data, "flappy.service") {
				found = true
			}
		default:
			time.Sleep(time.Millisecond)
		}
	}

	if !found {
		t.Error("expected a transition event mentioning flappy.service")
	}
}
