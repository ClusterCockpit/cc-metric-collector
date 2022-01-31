package collectors

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"strconv"
	"strings"
	"time"

	lp "github.com/ClusterCockpit/cc-metric-collector/internal/ccMetric"
)

const MEMSTATFILE = `/proc/meminfo`

type MemstatCollectorConfig struct {
	ExcludeMetrics []string `json:"exclude_metrics"`
}

type MemstatCollector struct {
	metricCollector
	stats   map[string]int64
	tags    map[string]string
	matches map[string]string
	config  MemstatCollectorConfig
}

func (m *MemstatCollector) Init(config json.RawMessage) error {
	var err error
	m.name = "MemstatCollector"
	if len(config) > 0 {
		err = json.Unmarshal(config, &m.config)
		if err != nil {
			return err
		}
	}
	m.meta = map[string]string{"source": m.name, "group": "Memory", "unit": "kByte"}
	m.stats = make(map[string]int64)
	m.matches = make(map[string]string)
	m.tags = map[string]string{"type": "node"}
	matches := map[string]string{`MemTotal`: "mem_total",
		"SwapTotal":    "swap_total",
		"SReclaimable": "mem_sreclaimable",
		"Slab":         "mem_slab",
		"MemFree":      "mem_free",
		"Buffers":      "mem_buffers",
		"Cached":       "mem_cached",
		"MemAvailable": "mem_available",
		"SwapFree":     "swap_free"}
	for k, v := range matches {
		_, skip := stringArrayContains(m.config.ExcludeMetrics, k)
		if !skip {
			m.matches[k] = v
		}
	}
	if len(m.matches) == 0 {
		return errors.New("No metrics to collect")
	}
	m.setup()
	_, err = ioutil.ReadFile(string(MEMSTATFILE))
	if err == nil {
		m.init = true
	}
	return err
}

func (m *MemstatCollector) Read(interval time.Duration, output chan *lp.CCMetric) {
	if !m.init {
		return
	}

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
			err = fmt.Errorf("Parse error for %s : %s", match, name)
			log.Print(err)
			continue
		}
		y, err := lp.New(name, m.tags, m.meta, map[string]interface{}{"value": int(float64(m.stats[match]) * 1.0e-3)}, time.Now())
		if err == nil {
			output <- &y
		}
	}

	if _, free := m.stats[`MemFree`]; free {
		if _, buffers := m.stats[`Buffers`]; buffers {
			if _, cached := m.stats[`Cached`]; cached {
				memUsed := m.stats[`MemTotal`] - (m.stats[`MemFree`] + m.stats[`Buffers`] + m.stats[`Cached`])
				_, skip := stringArrayContains(m.config.ExcludeMetrics, "mem_used")
				y, err := lp.New("mem_used", m.tags, m.meta, map[string]interface{}{"value": int(float64(memUsed) * 1.0e-3)}, time.Now())
				if err == nil && !skip {
					output <- &y
				}
			}
		}
	}
	if _, found := m.stats[`MemShared`]; found {
		_, skip := stringArrayContains(m.config.ExcludeMetrics, "mem_shared")
		y, err := lp.New("mem_shared", m.tags, m.meta, map[string]interface{}{"value": int(float64(m.stats[`MemShared`]) * 1.0e-3)}, time.Now())
		if err == nil && !skip {
			output <- &y
		}
	}
}

func (m *MemstatCollector) Close() {
	m.init = false
}
