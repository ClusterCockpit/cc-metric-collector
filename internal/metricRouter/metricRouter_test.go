package metricRouter

import (
	"encoding/json"
	"fmt"
	"sync"
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

func (t *fakeTicker) tick(ts time.Time) {
	for _, c := range t.channels {
		c <- ts
	}
}

func genMessages(t *testing.T, num int) []lp.CCMessage {
	t.Helper()
	msgs := make([]lp.CCMessage, 0, num)
	tags := map[string]string{"type": "node"}
	for i := range num {
		m, err := lp.NewMetric(fmt.Sprintf("testmetric%d", i), tags, nil, 42.0, time.Unix(1, 0))
		if err != nil {
			t.Fatalf("failed to create message: %s", err.Error())
		}
		msgs = append(msgs, m)
	}
	return msgs
}

// With interval_timestamp enabled, all metrics forwarded after a tick must
// carry that tick's timestamp, never the previous interval's
func TestIntervalTimestamp(t *testing.T) {
	ticker := &fakeTicker{}
	var wg sync.WaitGroup
	r, err := New(ticker, &wg, json.RawMessage(`{"interval_timestamp": true}`))
	if err != nil {
		t.Fatalf("failed to setup metric router: %s", err.Error())
	}

	coll := make(chan lp.CCMessage, 100)
	out := make(chan lp.CCMessage, 100)
	r.AddCollectorInput(coll)
	r.AddOutput(out)
	r.Start()

	receiveAll := func(num int) []lp.CCMessage {
		received := make([]lp.CCMessage, 0, num)
		for len(received) < num {
			select {
			case m := <-out:
				received = append(received, m)
			case <-time.After(5 * time.Second):
				t.Fatalf("received only %d of %d messages", len(received), num)
			}
		}
		return received
	}

	for interval, tickTime := range []time.Time{time.Unix(1000, 0), time.Unix(1010, 0)} {
		ticker.tick(tickTime)
		msgs := genMessages(t, 20)
		for _, m := range msgs {
			coll <- m
		}
		for i, m := range receiveAll(len(msgs)) {
			if !m.Time().Equal(tickTime) {
				t.Errorf("interval %d message %d: got timestamp %v, want %v", interval, i, m.Time(), tickTime)
			}
		}
	}

	closed := make(chan struct{})
	go func() {
		r.Close()
		close(closed)
	}()
	select {
	case <-closed:
	case <-time.After(5 * time.Second):
		t.Fatal("Close() did not terminate")
	}
	wg.Wait()
}
