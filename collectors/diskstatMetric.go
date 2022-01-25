package collectors

import (
	"io/ioutil"
	lp "github.com/ClusterCockpit/cc-metric-collector/internal/ccMetric"
	//	"log"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"
)

const DISKSTATFILE = `/proc/diskstats`
const DISKSTAT_SYSFSPATH = `/sys/block`

type DiskstatCollectorConfig struct {
	ExcludeMetrics []string `json:"exclude_metrics,omitempty"`
}

type DiskstatCollector struct {
	metricCollector
	matches map[int]string
	config  DiskstatCollectorConfig
}

func (m *DiskstatCollector) Init(config json.RawMessage) error {
	var err error
	m.name = "DiskstatCollector"
	m.meta = map[string]string{"source": m.name, "group": "Disk"}
	m.setup()
	if len(config) > 0 {
		err = json.Unmarshal(config, &m.config)
		if err != nil {
			return err
		}
	}
	// https://www.kernel.org/doc/html/latest/admin-guide/iostats.html
	matches := map[int]string{
		3:  "reads",
		4:  "reads_merged",
		5:  "read_sectors",
		6:  "read_ms",
		7:  "writes",
		8:  "writes_merged",
		9:  "writes_sectors",
		10: "writes_ms",
		11: "ioops",
		12: "ioops_ms",
		13: "ioops_weighted_ms",
		14: "discards",
		15: "discards_merged",
		16: "discards_sectors",
		17: "discards_ms",
		18: "flushes",
		19: "flushes_ms",
	}
	m.matches = make(map[int]string)
	for k, v := range matches {
		_, skip := stringArrayContains(m.config.ExcludeMetrics, v)
		if !skip {
			m.matches[k] = v
		}
	}
	if len(m.matches) == 0 {
		return errors.New("No metrics to collect")
	}
	_, err = ioutil.ReadFile(string(DISKSTATFILE))
	if err == nil {
		m.init = true
	}
	return err
}

func (m *DiskstatCollector) Read(interval time.Duration, output chan lp.CCMetric) {
	var lines []string
	if !m.init {
		return
	}

	buffer, err := ioutil.ReadFile(string(DISKSTATFILE))
	if err != nil {
		return
	}
	lines = strings.Split(string(buffer), "\n")

	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		f := strings.Fields(line)
		if strings.Contains(f[2], "loop") {
			continue
		}
		tags := map[string]string{
			"device": f[2],
			"type":   "node",
		}
		for idx, name := range m.matches {
			if idx < len(f) {
				x, err := strconv.ParseInt(f[idx], 0, 64)
				if err == nil {
					y, err := lp.New(name, tags, m.meta, map[string]interface{}{"value": int(x)}, time.Now())
					if err == nil {
						output <- y
					}
				}
			}
		}
	}
}

func (m *DiskstatCollector) Close() {
	m.init = false
}
