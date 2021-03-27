package collectors

import (
	"fmt"
	"io/ioutil"
	"log"
	"strconv"
	"strings"
	"time"
)

const NETSTATFILE = `/proc/net/dev`

type NetstatCollector struct {
	MetricCollector
}

func (m *NetstatCollector) Init() {
	m.name = "NetstatCollector"
	m.setup()
}

func (m *NetstatCollector) Read(interval time.Duration) {
	data, err := ioutil.ReadFile(string(NETSTATFILE))
	if err != nil {
		log.Print(err.Error())
		return
	}
	var matches = map[int]string{
		1:  "bytes_in",
		9:  "bytes_out",
		2:  "pkts_in",
		10: "pkts_out",
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
		for i, name := range matches {
			v, err := strconv.ParseInt(f[i], 10, 0)
			if err == nil {
				m.node[fmt.Sprintf("%s_%s", dev, name)] = float64(v) * 1.0e-3
			}
		}
	}

}

func (m *NetstatCollector) Close() {
	return
}
