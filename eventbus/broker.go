package eventbus

import (
	"log/slog"
	"sync"
	"time"

	"github.com/thesimonho/warden/event"
)

// clientBufferSize is the per-client event channel buffer.
// Events are dropped for slow clients to avoid blocking the broadcast goroutine.
const clientBufferSize = 64

// heartbeatInterval is how often a keepalive is sent to SSE clients.
const heartbeatInterval = 15 * time.Second

// Broker manages SSE client connections and fans out events.
//
// It maintains a set of subscriber channels. Broadcast sends to
// all subscribers; slow subscribers that can't keep up have events
// dropped rather than blocking.
type Broker struct {
	mu      sync.RWMutex
	clients map[chan event.SSEEvent]struct{}
	done    chan struct{}
}

// NewBroker creates and starts a new SSE broker.
// The heartbeat goroutine runs until Shutdown is called.
func NewBroker() *Broker {
	b := &Broker{
		clients: make(map[chan event.SSEEvent]struct{}),
		done:    make(chan struct{}),
	}
	go b.heartbeatLoop()
	return b
}

// Subscribe registers a new SSE client and returns its event channel
// and an unsubscribe function. The caller must call unsubscribe when
// the client disconnects.
func (b *Broker) Subscribe() (<-chan event.SSEEvent, func()) {
	ch := make(chan event.SSEEvent, clientBufferSize)

	b.mu.Lock()
	b.clients[ch] = struct{}{}
	b.mu.Unlock()

	unsubscribe := func() {
		b.mu.Lock()
		if _, ok := b.clients[ch]; ok {
			delete(b.clients, ch)
			close(ch)
		}
		b.mu.Unlock()
	}

	return ch, unsubscribe
}

// Broadcast sends an event to all connected clients.
// Slow clients have events dropped to avoid blocking.
func (b *Broker) Broadcast(evt event.SSEEvent) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for ch := range b.clients {
		select {
		case ch <- evt:
		default:
			slog.Debug("dropping SSE event for slow client", "event", evt.Event)
		}
	}
}

// ClientCount returns the current number of connected SSE clients.
func (b *Broker) ClientCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return len(b.clients)
}

// Shutdown stops the heartbeat goroutine and closes all client channels.
func (b *Broker) Shutdown() {
	close(b.done)

	b.mu.Lock()
	defer b.mu.Unlock()

	for ch := range b.clients {
		close(ch)
		delete(b.clients, ch)
	}
}

// heartbeatLoop sends periodic keepalive events to all clients.
func (b *Broker) heartbeatLoop() {
	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-b.done:
			return
		case <-ticker.C:
			b.Broadcast(event.SSEEvent{
				Event: event.SSEHeartbeat,
				Data:  []byte("{}"),
			})
		}
	}
}
