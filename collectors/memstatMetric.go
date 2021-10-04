package collectors

import (
	"errors"
	"fmt"
	lp "github.com/influxdata/line-protocol"
	"io/ioutil"
	"log"
	"strconv"
	"strings"
	"time"
)

const MEMSTATFILE = `/proc/meminfo`

type MemstatCollector struct {
	MetricCollector
	stats   map[string]int64
	tags    map[string]string
	matches map[string]string
}

func (m *MemstatCollector) Init() error {
	m.name = "MemstatCollector"
	m.stats = make(map[string]int64)
	m.tags = map[string]string{"type": "node"}
	m.matches = map[string]string{`MemTotal`: "mem_total",
		"SwapTotal":    "swap_total",
		"SReclaimable": "mem_sreclaimable",
		"Slab":         "mem_slab",
		"MemFree":      "mem_free",
		"Buffers":      "mem_buffers",
		"Cached":       "mem_cached",
		"MemAvailable": "mem_available",
		"SwapFree":     "swap_free"}
	m.setup()
	_, err := ioutil.ReadFile(string(MEMSTATFILE))
	if err == nil {
		m.init = true
	}
	return nil
}

func (m *MemstatCollector) Read(interval time.Duration, out *[]lp.MutableMetric) {
	buffer, err := ioutil.ReadFile(string(MEMSTATFILE))

	if err != nil {
		log.Print(err)
		return
	}

	ll := strings.Split(string(buffer), "\n")

	for _, line := range ll {
		ls := strings.Split(line, `:`)
		if len(ls) > 1 {
			lv := strings.Fields(ls[1])
			m.stats[ls[0]], err = strconv.ParseInt(lv[0], 0, 64)
		}
	}

	if _, exists := m.stats[`MemTotal`]; !exists {
		err = errors.New("Parse error")
		log.Print(err)
		return
	}

	for match, name := range m.matches {
		if _, exists := m.stats[match]; !exists {
			err = errors.New(fmt.Sprintf("Parse error for %s : %s", match, name))
			log.Print(err)
			continue
		}
		y, err := lp.New(name, m.tags, map[string]interface{}{"value": int(float64(m.stats[match]) * 1.0e-3)}, time.Now())
		if err == nil {
			*out = append(*out, y)
		}
	}

	if _, free := m.stats[`MemFree`]; free {
		if _, buffers := m.stats[`Buffers`]; buffers {
			if _, cached := m.stats[`Cached`]; cached {
				memUsed := m.stats[`MemTotal`] - (m.stats[`MemFree`] + m.stats[`Buffers`] + m.stats[`Cached`])
				y, err := lp.New("mem_used", m.tags, map[string]interface{}{"value": int(float64(memUsed) * 1.0e-3)}, time.Now())
				if err == nil {
					*out = append(*out, y)
				}
			}
		}
	}
	if _, found := m.stats[`MemShared`]; found {
		y, err := lp.New("mem_shared", m.tags, map[string]interface{}{"value": int(float64(m.stats[`MemShared`]) * 1.0e-3)}, time.Now())
		if err == nil {
			*out = append(*out, y)
		}
	}
}

func (m *MemstatCollector) Close() {
	m.init = false
	return
}
