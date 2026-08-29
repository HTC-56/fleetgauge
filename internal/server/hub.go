package server

import "sync"

// subscriberBuffer is how many events a single SSE subscriber may fall behind
// before it starts missing them. Sixteen is roughly eight seconds of slack at
// the default broadcast interval — long enough to ride out a garbage
// collection or a slow network write, short enough that a suspended browser
// tab holds only kilobytes.
const subscriberBuffer = 16

// Event is one server-sent event: an SSE event name and its already-encoded
// data payload. The Hub does no encoding — it moves strings.
type Event struct {
	Name string
	Data string
}

// Hub fans one publisher out to many SSE subscribers.
//
// Each subscriber owns a buffered channel. A subscriber that stops reading is
// dropped from that broadcast rather than blocked on: a browser tab that
// suspends must not be able to stall the broadcast loop or wedge every other
// viewer. Publish therefore never blocks and never fails.
//
// Hub is safe for concurrent use.
type Hub struct {
	mu      sync.Mutex
	subs    map[chan Event]struct{}
	closed  bool
	dropped int
}

// NewHub returns an empty Hub, ready for subscribers.
func NewHub() *Hub {
	return &Hub{subs: make(map[chan Event]struct{})}
}

// Subscribe registers a subscriber and returns its event channel together with
// the cancel function that unregisters it. Every caller must call cancel —
// deferring it is the norm — or the subscriber leaks for the life of the Hub.
//
// cancel is idempotent and closes the channel, so a receive loop ranging over
// it terminates. Subscribing to a closed Hub yields an already-closed channel
// and a no-op cancel, so a request arriving during shutdown ends immediately
// instead of hanging.
func (h *Hub) Subscribe() (<-chan Event, func()) {
	h.mu.Lock()
	defer h.mu.Unlock()

	ch := make(chan Event, subscriberBuffer)
	if h.closed {
		close(ch)
		return ch, func() {}
	}
	h.subs[ch] = struct{}{}

	var once sync.Once
	cancel := func() {
		once.Do(func() {
			h.mu.Lock()
			defer h.mu.Unlock()
			// Close may already have removed and closed this channel; the
			// membership check is what makes the double-close impossible.
			if _, ok := h.subs[ch]; ok {
				delete(h.subs, ch)
				close(ch)
			}
		})
	}
	return ch, cancel
}

// Publish delivers ev to every current subscriber. It never blocks: a
// subscriber whose buffer is full misses this event and the drop counter
// advances. Publishing to a closed Hub is a no-op.
func (h *Hub) Publish(ev Event) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.closed {
		return
	}
	for ch := range h.subs {
		select {
		case ch <- ev:
		default:
			h.dropped++
		}
	}
}

// Count reports the number of live subscribers.
func (h *Hub) Count() int {
	h.mu.Lock()
	defer h.mu.Unlock()

	return len(h.subs)
}

// Dropped reports how many events have been discarded because a subscriber's
// buffer was full. It is monotonic, and it is the honest measure of whether
// subscriberBuffer is big enough in practice.
func (h *Hub) Dropped() int {
	h.mu.Lock()
	defer h.mu.Unlock()

	return h.dropped
}

// Close unregisters and closes every subscriber channel. Later Publish calls
// are no-ops and later Subscribe calls return an already-closed channel.
// Close is idempotent.
func (h *Hub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.closed {
		return
	}
	h.closed = true
	for ch := range h.subs {
		delete(h.subs, ch)
		close(ch)
	}
}
