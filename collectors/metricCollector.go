package collectors

import (
	"log"
	"os"
	"time"

	protocol "github.com/influxdata/line-protocol"
)

type MetricCollector struct {
	name       string
	tags       []*protocol.Tag
	fields     []*protocol.Field
	t          time.Time
	serializer *protocol.Encoder
	done       chan bool
	ticker     *time.Ticker
}

type MetricGetter interface {
	Init()
	Start(time.Duration)
	Stop()
}

func (c *MetricCollector) Name() string {
	return c.name
}

func (c *MetricCollector) TagList() []*protocol.Tag {
	return c.tags
}

func (c *MetricCollector) FieldList() []*protocol.Field {
	return c.fields
}

func (c *MetricCollector) Time() time.Time {
	return c.t
}

func (c *MetricCollector) startLoop(
	interval time.Duration,
	getMetrics func() error) {

	log.Print("Start loop")
	c.ticker = time.NewTicker(interval * time.Second)

	go func() {
		for {
			select {
			case <-c.done:
				return
			case c.t = <-c.ticker.C:
				getMetrics()
				_, err := c.serializer.Encode(c)
				if err != nil {
					log.Print(err)
				}
			}
		}
	}()
}

func (c *MetricCollector) setup() {
	hostname, _ := os.Hostname()
	c.serializer = protocol.NewEncoder(os.Stdout)
	c.serializer.SetPrecision(time.Second)
	c.serializer.SetMaxLineBytes(1024)
	c.tags = make([]*protocol.Tag, 1)
	c.tags[0] = &protocol.Tag{Key: "host", Value: hostname}
	c.name = "node"
	c.done = make(chan bool)
}

func (c *MetricCollector) Stop() {
	c.ticker.Stop()
	c.done <- true
}
