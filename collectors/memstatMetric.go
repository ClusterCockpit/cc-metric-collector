package collectors

import (
	"errors"
	"io/ioutil"
	"log"
	"strconv"
	"strings"
	"time"

	//protocol "github.com/influxdata/line-protocol"
)

const MEMSTATFILE = `/proc/meminfo`

type MemstatCollector struct {
	MetricCollector
}


func (m *MemstatCollector) Init() {
    m.name = "MemstatCollector"
	m.setup()
}

func (m *MemstatCollector) Read(interval time.Duration){
	buffer, err := ioutil.ReadFile(string(MEMSTATFILE))

	if err != nil {
		log.Print(err)
		return
	}

	ll := strings.Split(string(buffer), "\n")
	memstats := make(map[string]int64)

	for _, line := range ll {
		ls := strings.Split(line, `:`)
		if len(ls) > 1 {
			lv := strings.Fields(ls[1])
			memstats[ls[0]], err = strconv.ParseInt(lv[0], 0, 64)
		}
	}

	if _, exists := memstats[`MemTotal`]; !exists {
		err = errors.New("Parse error")
		log.Print(err)
		return
	}

	memUsed := memstats[`MemTotal`] - (memstats[`MemFree`] + memstats[`Buffers`] + memstats[`Cached`])
	m.node["mem_used"] = float64(memUsed) * 1.0e-3
}

func (m *MemstatCollector) Close() {
    return
}
