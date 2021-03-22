package collectors

import (
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"
	"time"

	protocol "github.com/influxdata/line-protocol"
)

const NETSTATFILE = `/proc/net/dev`

type NetstatCollector struct {
	MetricCollector
}

func (m *NetstatCollector) parse() (err error) {
	data, err := ioutil.ReadFile(string(NETSTATFILE))
	if err != nil {
		fmt.Println(err.Error())
		return err
	}
	var matches = map[int]int{
		1:  0, // "bytes_in",
		9:  1, // "bytes_out",
		2:  2, // "pkts_in",
		10: 3} // "pkts_out",

	lines := strings.Split(string(data), "\n")
	for _, l := range lines {
		if !strings.Contains(l, ":") {
			continue
		}
		f := strings.Fields(l)
		dev := f[0][0 : len(f[0])-1]
		if dev == "lo" {
			continue
		}
		for i, ind := range matches {
			v, err := strconv.ParseInt(f[i], 10, 0)
			if err == nil {
				m.fields[ind].Value = float64(v) * 1.0e-3
			}
		}
	}
	return nil
}

func (m *NetstatCollector) Init() {
	m.setup()
	m.fields = make([]*protocol.Field, 4)
	m.fields[0] = &protocol.Field{Key: "bytes_in", Value: 0}
	m.fields[1] = &protocol.Field{Key: "bytes_out", Value: 0}
	m.fields[2] = &protocol.Field{Key: "pkts_in", Value: 0}
	m.fields[3] = &protocol.Field{Key: "pkts_out", Value: 0}
}

func (m *NetstatCollector) Start(interval time.Duration) {
	m.startLoop(interval, m.parse)
}
