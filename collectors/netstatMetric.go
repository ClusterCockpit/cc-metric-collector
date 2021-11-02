package collectors

import (
	lp "github.com/influxdata/line-protocol"
	"io/ioutil"
	"log"
	"strconv"
	"strings"
	"time"
)

const NETSTATFILE = `/proc/net/dev`

type NetstatCollector struct {
	MetricCollector
	matches map[int]string
	tags    map[string]string
}

func (m *NetstatCollector) Init() error {
	m.name = "NetstatCollector"
	m.setup()
	m.tags = map[string]string{"type": "node"}
	m.matches = map[int]string{
		1:  "bytes_in",
		9:  "bytes_out",
		2:  "pkts_in",
		10: "pkts_out",
	}
	_, err := ioutil.ReadFile(string(NETSTATFILE))
	if err == nil {
		m.init = true
	}
	return nil
}

func (m *NetstatCollector) Read(interval time.Duration, out *[]lp.MutableMetric) {
	data, err := ioutil.ReadFile(string(NETSTATFILE))
	if err != nil {
		log.Print(err.Error())
		return
	}

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
		for i, name := range m.matches {
			v, err := strconv.ParseInt(f[i], 10, 0)
			if err == nil {
				y, err := lp.New(name, m.tags, map[string]interface{}{"value": int(float64(v) * 1.0e-3)}, time.Now())
				if err == nil {
					*out = append(*out, y)
				}
			}
		}
	}

}

func (m *NetstatCollector) Close() {
	m.init = false
	return
}
