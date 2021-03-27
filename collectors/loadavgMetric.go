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

func (m *LoadavgCollector) Init() {
	m.name = "LoadavgCollector"
	m.setup()
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
}

func (m *LoadavgCollector) Close() {
	return
}
