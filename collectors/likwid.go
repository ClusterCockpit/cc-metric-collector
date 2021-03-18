package collectors

import (
	"bytes"
	"fmt"
	"time"

	protocol "github.com/influxdata/line-protocol"
)

type LikwidCollector struct {
	name    string
	tags    []*protocol.Tag
	fields  []*protocol.Field
	t       time.Time
	encoder *protocol.Encoder
}

func (c *LikwidCollector) Name() string {
	return c.name
}
func (c *LikwidCollector) TagList() []*protocol.Tag {
	return c.tags
}

func (c *LikwidCollector) FieldList() []*protocol.Field {
	return c.fields
}

func (c *LikwidCollector) Time() time.Time {
	return c.t
}

func (c *LikwidCollector) New() {
	buf := &bytes.Buffer{}
	c.encoder = protocol.NewEncoder(buf)
	c.encoder.SetMaxLineBytes(1024)
}

func (c *LikwidCollector) Start(
	level string,
	frequency time.Duration,
	duration int) {
	ticker := time.NewTicker(frequency * time.Second)
	done := make(chan bool)

	go func() {
		for {
			select {
			case <-done:
				return
			case t := <-ticker.C:
				fmt.Println("Tick at", t)

				c.encoder.Encode(c)
			}
		}
	}()

	time.Sleep(1600 * time.Second)
	ticker.Stop()
	done <- true
	fmt.Println("Ticker stopped")
}
