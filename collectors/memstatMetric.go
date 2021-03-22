package collectors

import (
	"errors"
	"io/ioutil"
	"log"
	"strconv"
	s "strings"
	"time"

	protocol "github.com/influxdata/line-protocol"
)

const MEMSTATFILE = `/proc/meminfo`

type MemstatCollector struct {
	MetricCollector
}

func (m *MemstatCollector) parse() (err error) {

	buffer, err := ioutil.ReadFile(string(MEMSTATFILE))

	if err != nil {
		log.Print(err)
		return err
	}

	ll := s.Split(string(buffer), "\n")
	memstats := make(map[string]int64)

	for _, line := range ll {
		ls := s.Split(line, `:`)
		if len(ls) > 1 {
			lv := s.Fields(ls[1])
			memstats[ls[0]], err = strconv.ParseInt(lv[0], 0, 64)
		}
	}

	if _, exists := memstats[`MemTotal`]; !exists {
		err = errors.New("Parse error")
		log.Print(err)
		return err
	}

	memUsed := memstats[`MemTotal`] - (memstats[`MemFree`] + memstats[`Buffers`] + memstats[`Cached`])
	m.fields[0].Value = float64(memUsed) * 1.0e-3
	return nil
}

func (m *MemstatCollector) Init() {
	m.setup()
	m.fields = make([]*protocol.Field, 1)
	m.fields[0] = &protocol.Field{Key: "mem_used", Value: 0}
}

func (m *MemstatCollector) Start(interval time.Duration) {
	m.startLoop(interval, m.parse)
}
