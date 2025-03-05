package collectors

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	lp "github.com/ClusterCockpit/cc-lib/ccMessage"
	cclog "github.com/ClusterCockpit/cc-metric-collector/pkg/ccLogger"
)

// LoadavgCollector collects:
// * load average of last 1, 5 & 15 minutes
// * number of processes currently runnable
// * total number of processes in system
//
// See: https://www.kernel.org/doc/html/latest/filesystems/proc.html
const LOADAVGFILE = "/proc/loadavg"

type LoadavgCollector struct {
	metricCollector
	tags   map[string]string
	config struct {
		ExcludeMetrics []string `json:"exclude_metrics,omitempty"`
		OnlyMetrics    []string `json:"only_metrics,omitempty"`
	}
}

func (m *LoadavgCollector) shouldOutput(metricName string) bool {
	if len(m.config.OnlyMetrics) > 0 {
		for _, n := range m.config.OnlyMetrics {
			if n == metricName {
				return true
			}
		}
		return false
	}
	for _, n := range m.config.ExcludeMetrics {
		if n == metricName {
			return false
		}
	}
	return true
}

func (m *LoadavgCollector) Init(config json.RawMessage) error {
	m.name = "LoadavgCollector"
	m.parallel = true
	m.setup()
	if len(config) > 0 {
		if err := json.Unmarshal(config, &m.config); err != nil {
			return err
		}
	}
	m.meta = map[string]string{"source": m.name, "group": "LOAD"}
	m.tags = map[string]string{"type": "node"}
	m.init = true
	return nil
}

func (m *LoadavgCollector) Read(interval time.Duration, output chan lp.CCMessage) {
	if !m.init {
		return
	}
	buffer, err := os.ReadFile(LOADAVGFILE)
	if err != nil {
		cclog.ComponentError(m.name, fmt.Sprintf("Read(): Failed to read file '%s': %v", LOADAVGFILE, err))
		return
	}
	now := time.Now()
	ls := strings.Split(string(buffer), " ")

	// Load metrics
	loadMetrics := []string{"load_one", "load_five", "load_fifteen"}
	for i, name := range loadMetrics {
		x, err := strconv.ParseFloat(ls[i], 64)
		if err != nil {
			cclog.ComponentError(m.name, fmt.Sprintf("Read(): Failed to convert '%s' to float64: %v", ls[i], err))
			continue
		}
		if m.shouldOutput(name) {
			y, err := lp.NewMessage(name, m.tags, m.meta, map[string]interface{}{"value": x}, now)
			if err == nil {
				output <- y
			}
		}
	}

	// Process metrics
	lv := strings.Split(ls[3], `/`)
	procMetrics := []string{"proc_run", "proc_total"}
	for i, name := range procMetrics {
		x, err := strconv.ParseInt(lv[i], 10, 64)
		if err != nil {
			cclog.ComponentError(m.name, fmt.Sprintf("Read(): Failed to convert '%s' to int64: %v", lv[i], err))
			continue
		}
		if m.shouldOutput(name) {
			y, err := lp.NewMessage(name, m.tags, m.meta, map[string]interface{}{"value": x}, now)
			if err == nil {
				output <- y
			}
		}
	}
}

func (m *LoadavgCollector) Close() {
	m.init = false
}
