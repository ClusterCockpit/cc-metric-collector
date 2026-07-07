package multiChanTicker

import (
	"testing"
	"time"
)

// A consumer that never reads its channel must not stall the ticker
// or starve the other consumers
func TestStalledConsumerDoesNotStarveOthers(t *testing.T) {
	stalled := make(chan time.Time, 1) // never read
	fast := make(chan time.Time, 1)

	ticker := NewTicker(10 * time.Millisecond)
	defer ticker.Close()
	ticker.AddChannel(stalled)
	ticker.AddChannel(fast)

	received := 0
	deadline := time.After(5 * time.Second)
	for received < 5 {
		select {
		case <-fast:
			received++
		case <-deadline:
			t.Fatalf("received only %d ticks while another consumer stalled", received)
		}
	}
}

// Close() must return promptly even if no consumer reads its channel
func TestCloseWithStalledConsumer(t *testing.T) {
	stalled := make(chan time.Time, 1) // never read

	ticker := NewTicker(10 * time.Millisecond)
	ticker.AddChannel(stalled)

	// Let some ticks fire and be dropped
	time.Sleep(50 * time.Millisecond)

	closed := make(chan struct{})
	go func() {
		ticker.Close()
		close(closed)
	}()
	select {
	case <-closed:
	case <-time.After(5 * time.Second):
		t.Fatal("Close() blocked with a stalled consumer")
	}
}
