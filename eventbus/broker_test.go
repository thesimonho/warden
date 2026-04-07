package eventbus

import (
	"testing"
	"time"

	"github.com/thesimonho/warden/event"
)

func TestBroker_SubscribeAndBroadcast(t *testing.T) {
	b := NewBroker()
	defer b.Shutdown()

	ch, unsub := b.Subscribe()
	defer unsub()

	evt := event.SSEEvent{Event: event.SSEWorktreeState, Data: []byte(`{"test":true}`)}
	b.Broadcast(evt)

	select {
	case got := <-ch:
		if got.Event != event.SSEWorktreeState {
			t.Errorf("expected worktree_state, got %s", got.Event)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for broadcast")
	}
}

func TestBroker_MultipleClients(t *testing.T) {
	b := NewBroker()
	defer b.Shutdown()

	ch1, unsub1 := b.Subscribe()
	defer unsub1()
	ch2, unsub2 := b.Subscribe()
	defer unsub2()

	evt := event.SSEEvent{Event: event.SSEProjectState, Data: []byte(`{}`)}
	b.Broadcast(evt)

	for _, ch := range []<-chan event.SSEEvent{ch1, ch2} {
		select {
		case got := <-ch:
			if got.Event != event.SSEProjectState {
				t.Errorf("expected project_state, got %s", got.Event)
			}
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for broadcast")
		}
	}
}

func TestBroker_Unsubscribe(t *testing.T) {
	b := NewBroker()
	defer b.Shutdown()

	ch, unsub := b.Subscribe()

	if b.ClientCount() != 1 {
		t.Errorf("expected 1 client, got %d", b.ClientCount())
	}

	unsub()

	if b.ClientCount() != 0 {
		t.Errorf("expected 0 clients after unsubscribe, got %d", b.ClientCount())
	}

	// Channel should be drained by unsub, this read should not block.
	_ = ch
}

func TestBroker_SlowClientDoesNotBlock(t *testing.T) {
	b := NewBroker()
	defer b.Shutdown()

	_, unsub := b.Subscribe() // Subscribe but never read
	defer unsub()

	// Flood the channel beyond buffer size — should not block.
	done := make(chan struct{})
	go func() {
		for range clientBufferSize + 10 {
			b.Broadcast(event.SSEEvent{Event: event.SSEHeartbeat, Data: []byte(`{}`)})
		}
		close(done)
	}()

	select {
	case <-done:
		// Success — didn't block.
	case <-time.After(2 * time.Second):
		t.Fatal("broadcast blocked on slow client")
	}
}

func TestBroker_Heartbeat(t *testing.T) {
	// Create broker with a very short heartbeat for testing.
	b := &Broker{
		clients: make(map[chan event.SSEEvent]struct{}),
		done:    make(chan struct{}),
	}
	// Start a custom heartbeat loop at a test-friendly interval.
	go func() {
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-b.done:
				return
			case <-ticker.C:
				b.Broadcast(event.SSEEvent{Event: event.SSEHeartbeat, Data: []byte(`{}`)})
			}
		}
	}()
	defer b.Shutdown()

	ch, unsub := b.Subscribe()
	defer unsub()

	select {
	case got := <-ch:
		if got.Event != event.SSEHeartbeat {
			t.Errorf("expected heartbeat, got %s", got.Event)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for heartbeat")
	}
}

func TestBroker_ShutdownClosesChannels(t *testing.T) {
	b := NewBroker()

	ch, _ := b.Subscribe()
	b.Shutdown()

	// Channel should be closed after shutdown.
	_, ok := <-ch
	if ok {
		t.Error("expected channel to be closed after shutdown")
	}
}

func TestBroker_ClientCount(t *testing.T) {
	b := NewBroker()
	defer b.Shutdown()

	if b.ClientCount() != 0 {
		t.Errorf("expected 0, got %d", b.ClientCount())
	}

	_, unsub1 := b.Subscribe()
	_, unsub2 := b.Subscribe()

	if b.ClientCount() != 2 {
		t.Errorf("expected 2, got %d", b.ClientCount())
	}

	unsub1()

	if b.ClientCount() != 1 {
		t.Errorf("expected 1, got %d", b.ClientCount())
	}

	unsub2()

	if b.ClientCount() != 0 {
		t.Errorf("expected 0, got %d", b.ClientCount())
	}
}
