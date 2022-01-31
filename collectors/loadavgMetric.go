package collectors

import (
	"encoding/json"
	"io/ioutil"
	"strconv"
	"strings"
	"time"

	lp "github.com/ClusterCockpit/cc-metric-collector/internal/ccMetric"
)

const LOADAVGFILE = `/proc/loadavg`

type LoadavgCollectorConfig struct {
	ExcludeMetrics []string `json:"exclude_metrics,omitempty"`
}

type LoadavgCollector struct {
	metricCollector
	tags         map[string]string
	load_matches []string
	proc_matches []string
	config       LoadavgCollectorConfig
}

func (m *LoadavgCollector) Init(config json.RawMessage) error {
	m.name = "LoadavgCollector"
	m.setup()
	if len(config) > 0 {
		err := json.Unmarshal(config, &m.config)
		if err != nil {
			return err
		}
	}
	m.meta = map[string]string{"source": m.name, "group": "LOAD"}
	m.tags = map[string]string{"type": "node"}
	m.load_matches = []string{"load_one", "load_five", "load_fifteen"}
	m.proc_matches = []string{"proc_run", "proc_total"}
	m.init = true
	return nil
}

func (m *LoadavgCollector) Read(interval time.Duration, output chan *lp.CCMetric) {
	var skip bool
	if !m.init {
		return
	}
	buffer, err := ioutil.ReadFile(string(LOADAVGFILE))

	if err != nil {
		return
	}

	ls := strings.Split(string(buffer), ` `)
	for i, name := range m.load_matches {
		x, err := strconv.ParseFloat(ls[i], 64)
		if err == nil {
			_, skip = stringArrayContains(m.config.ExcludeMetrics, name)
			y, err := lp.New(name, m.tags, m.meta, map[string]interface{}{"value": float64(x)}, time.Now())
			if err == nil && !skip {
				output <- &y
			}
		}
	}
	lv := strings.Split(ls[3], `/`)
	for i, name := range m.proc_matches {
		x, err := strconv.ParseFloat(lv[i], 64)
		if err == nil {
			_, skip = stringArrayContains(m.config.ExcludeMetrics, name)
			y, err := lp.New(name, m.tags, m.meta, map[string]interface{}{"value": float64(x)}, time.Now())
			if err == nil && !skip {
				output <- &y
			}
		}
	}
}

func (m *LoadavgCollector) Close() {
	m.init = false
}
