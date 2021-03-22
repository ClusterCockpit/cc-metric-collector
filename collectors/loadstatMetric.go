package collectors

import (
	"io/ioutil"
	"strconv"
	s "strings"
	"time"

	protocol "github.com/influxdata/line-protocol"
)

const LOADSTATFILE = `/proc/loadavg`

type LoadstatCollector struct {
	MetricCollector
}

func (m *LoadstatCollector) parse() (err error) {

	buffer, err := ioutil.ReadFile(string(LOADSTATFILE))

	if err != nil {
		return err
	}

	ls := s.Split(string(buffer), ` `)
	loadOne, _ := strconv.ParseFloat(ls[0], 32)
	m.fields[0].Value = float64(loadOne)
	return nil
}

func (m *LoadstatCollector) Init() {
	m.setup()
	m.fields = make([]*protocol.Field, 1)
	m.fields[0] = &protocol.Field{Key: "load", Value: 0}
}

func (m *LoadstatCollector) Start(interval time.Duration) {
	m.startLoop(interval, m.parse)
}
