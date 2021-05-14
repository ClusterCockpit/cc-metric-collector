package collectors

import (
	"io/ioutil"
	"strconv"
	"strings"
	"time"
)

const LOADAVGFILE = `/proc/loadavg`

type LoadavgCollector struct {
	MetricCollector
}

func (m *LoadavgCollector) Init() error {
	m.name = "LoadavgCollector"
	m.setup()
	return nil
}

func (m *LoadavgCollector) Read(interval time.Duration) {
	buffer, err := ioutil.ReadFile(string(LOADAVGFILE))

	if err != nil {
		return
	}

	ls := strings.Split(string(buffer), ` `)
	loadOne, _ := strconv.ParseFloat(ls[0], 64)
	m.node["load_one"] = float64(loadOne)
	loadFive, _ := strconv.ParseFloat(ls[1], 64)
	m.node["load_five"] = float64(loadFive)
	loadFifteen, _ := strconv.ParseFloat(ls[2], 64)
	m.node["load_fifteen"] = float64(loadFifteen)
	lv := strings.Split(ls[3], `/`)
	proc_run, _ := strconv.ParseFloat(lv[0], 64)
	proc_total, _ := strconv.ParseFloat(lv[1], 64)
	m.node["proc_total"] = float64(proc_total)
	m.node["proc_run"] = float64(proc_run)
}

func (m *LoadavgCollector) Close() {
	return
}
