package collectors

import (
	"io/ioutil"
	"strconv"
	"time"
	"strings"
)

const LOADAVGFILE = `/proc/loadavg`

type LoadavgCollector struct {
	MetricCollector
}


func (m *LoadavgCollector) Init() {
    m.name = "LoadavgCollector"
	m.setup()
}

func (m *LoadavgCollector) Read(interval time.Duration){
	buffer, err := ioutil.ReadFile(string(LOADAVGFILE))

	if err != nil {
		return
	}

	ls := strings.Split(string(buffer), ` `)
	loadOne, _ := strconv.ParseFloat(ls[0], 64)
	m.node["load_one"] = float64(loadOne)
}

func (m *LoadavgCollector) Close() {
    return
}
