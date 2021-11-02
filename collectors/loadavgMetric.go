package collectors

import (
	lp "github.com/influxdata/line-protocol"
	"io/ioutil"
	"strconv"
	"strings"
	"time"
)

const LOADAVGFILE = `/proc/loadavg`

type LoadavgCollector struct {
	MetricCollector
	tags         map[string]string
	load_matches []string
	proc_matches []string
}

func (m *LoadavgCollector) Init() error {
	m.name = "LoadavgCollector"
	m.setup()
	m.tags = map[string]string{"type": "node"}
	m.load_matches = []string{"load_one", "load_five", "load_fifteen"}
	m.proc_matches = []string{"proc_run", "proc_total"}
	m.init = true
	return nil
}

func (m *LoadavgCollector) Read(interval time.Duration, out *[]lp.MutableMetric) {

	buffer, err := ioutil.ReadFile(string(LOADAVGFILE))

	if err != nil {
		return
	}

	ls := strings.Split(string(buffer), ` `)
	for i, name := range m.load_matches {
		x, err := strconv.ParseFloat(ls[i], 64)
		if err == nil {
			y, err := lp.New(name, m.tags, map[string]interface{}{"value": float64(x)}, time.Now())
			if err == nil {
				*out = append(*out, y)
			}
		}
	}
	lv := strings.Split(ls[3], `/`)
	for i, name := range m.proc_matches {
		x, err := strconv.ParseFloat(lv[i], 64)
		if err == nil {
			y, err := lp.New(name, m.tags, map[string]interface{}{"value": float64(x)}, time.Now())
			if err == nil {
				*out = append(*out, y)
			}
		}
	}
}

func (m *LoadavgCollector) Close() {
	m.init = false
	return
}
