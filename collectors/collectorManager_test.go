package collectors

import (
	"encoding/json"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	lp "github.com/ClusterCockpit/cc-lib/v2/ccMessage"
)

// Fake ticker that delivers ticks on demand
type fakeTicker struct {
	channels []chan time.Time
}

func (t *fakeTicker) Init(duration time.Duration) {}

func (t *fakeTicker) AddChannel(c chan time.Time) {
	t.channels = append(t.channels, c)
}

func (t *fakeTicker) Close() {}

func (t *fakeTicker) tick() {
	for _, c := range t.channels {
		select {
		case c <- time.Now():
		default:
		}
	}
}

// Stub collector whose Read blocks until it is released
type stubCollector struct {
	metricCollector
	readStarted chan struct{}
	release     chan struct{}
	reads       atomic.Int32
}

func (c *stubCollector) Init(config json.RawMessage) error {
	c.name = "teststub"
	c.parallel = true
	c.init = true
	return nil
}

func (c *stubCollector) Read(duration time.Duration, output chan lp.CCMessage) {
	c.reads.Add(1)
	c.readStarted <- struct{}{}
	<-c.release
}

func (c *stubCollector) Close() {}

func TestOverlongCollectionRoundSkipsTick(t *testing.T) {
	stub := &stubCollector{
		readStarted: make(chan struct{}, 10),
		release:     make(chan struct{}),
	}
	AvailableCollectors["teststub"] = stub
	defer delete(AvailableCollectors, "teststub")

	ticker := &fakeTicker{}
	var wg sync.WaitGroup
	cm, err := New(ticker, time.Second, &wg, json.RawMessage(`{"teststub": {}}`))
	if err != nil {
		t.Fatalf("failed to setup collector manager: %s", err.Error())
	}
	cm.AddOutput(make(chan lp.CCMessage, 100))
	cm.Start()

	// First tick starts a collection round that blocks in Read
	ticker.tick()
	select {
	case <-stub.readStarted:
	case <-time.After(5 * time.Second):
		t.Fatal("collection round did not start on tick")
	}

	// Further ticks while the round is still running must be skipped,
	// not queued up or run concurrently
	for range 3 {
		ticker.tick()
		time.Sleep(20 * time.Millisecond)
	}
	if got := stub.reads.Load(); got != 1 {
		t.Fatalf("expected 1 concurrent collection round, got %d reads", got)
	}

	// Finish the round, the next tick must start a new one
	stub.release <- struct{}{}
	deadline := time.After(5 * time.Second)
	for stub.reads.Load() < 2 {
		ticker.tick()
		select {
		case <-stub.readStarted:
		case <-time.After(20 * time.Millisecond):
		case <-deadline:
			t.Fatal("no new collection round started after the previous one finished")
		}
	}

	// Shutdown must wait for the running round and terminate cleanly.
	// Closing the release channel lets any still running or straggler
	// round finish immediately
	close(stub.release)
	closed := make(chan struct{})
	go func() {
		cm.Close()
		close(closed)
	}()
	select {
	case <-closed:
	case <-time.After(5 * time.Second):
		t.Fatal("Close() did not terminate")
	}
	wg.Wait()
}
