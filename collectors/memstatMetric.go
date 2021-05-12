package collectors

import (
	"errors"
	"io/ioutil"
	"log"
	"strconv"
	"strings"
	"time"
)

const MEMSTATFILE = `/proc/meminfo`

type MemstatCollector struct {
	MetricCollector
}

func (m *MemstatCollector) Init() {
	m.name = "MemstatCollector"
	m.setup()
}

func (m *MemstatCollector) Read(interval time.Duration) {
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

	m.node["mem_total"] = float64(memstats[`MemTotal`]) * 1.0e-3
	m.node["swap_total"] = float64(memstats[`SwapTotal`]) * 1.0e-3
	m.node["mem_sreclaimable"] = float64(memstats[`SReclaimable`]) * 1.0e-3
	m.node["mem_slab"] = float64(memstats[`Slab`]) * 1.0e-3
	m.node["mem_free"] = float64(memstats[`MemFree`]) * 1.0e-3
	m.node["mem_buffers"] = float64(memstats[`Buffers`]) * 1.0e-3
	m.node["mem_cached"] = float64(memstats[`Cached`]) * 1.0e-3
	m.node["mem_available"] = float64(memstats[`MemAvailable`]) * 1.0e-3
	m.node["swap_free"] = float64(memstats[`SwapFree`]) * 1.0e-3

	memUsed := memstats[`MemTotal`] - (memstats[`MemFree`] + memstats[`Buffers`] + memstats[`Cached`])
	m.node["mem_used"] = float64(memUsed) * 1.0e-3
	// In linux-2.5.52 when Memshared was removed
	if _, found := memstats[`MemShared`]; found {
		m.node["mem_shared"] = float64(memstats[`MemShared`]) * 1.0e-3
	}
}

func (m *MemstatCollector) Close() {
	return
}
