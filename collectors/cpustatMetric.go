package collectors

import (
	"encoding/json"
	"fmt"
	lp "github.com/influxdata/line-protocol"
	"io/ioutil"
	"strconv"
	"strings"
	"time"
)

const CPUSTATFILE = `/proc/stat`

type CpustatCollectorConfig struct {
	ExcludeMetrics []string `json:"exclude_metrics, omitempty"`
}

type CpustatCollector struct {
	MetricCollector
	config CpustatCollectorConfig
}

func (m *CpustatCollector) Init(config []byte) error {
	m.name = "CpustatCollector"
	m.setup()
	if len(config) > 0 {
		err := json.Unmarshal(config, &m.config)
		if err != nil {
			return err
		}
	}
	m.init = true
	return nil
}

func ParseStatLine(line string, cpu int, exclude []string, out *[]lp.MutableMetric) {
	ls := strings.Fields(line)
	matches := []string{"", "cpu_user", "cpu_nice", "cpu_system", "cpu_idle", "cpu_iowait", "cpu_irq", "cpu_softirq", "cpu_steal", "cpu_guest", "cpu_guest_nice"}
	for _, ex := range exclude {
		matches, _ = RemoveFromStringList(matches, ex)
	}

	var tags map[string]string
	if cpu < 0 {
		tags = map[string]string{"type": "node"}
	} else {
		tags = map[string]string{"type": "cpu", "type-id": fmt.Sprintf("%d", cpu)}
	}
	for i, m := range matches {
		if len(m) > 0 {
			x, err := strconv.ParseInt(ls[i], 0, 64)
			if err == nil {
				y, err := lp.New(m, tags, map[string]interface{}{"value": int(x)}, time.Now())
				if err == nil {
					*out = append(*out, y)
				}
			}
		}
	}
}

func (m *CpustatCollector) Read(interval time.Duration, out *[]lp.MutableMetric) {
	if !m.init {
		return
	}
	buffer, err := ioutil.ReadFile(string(CPUSTATFILE))

	if err != nil {
		return
	}

	ll := strings.Split(string(buffer), "\n")
	for _, line := range ll {
		if len(line) == 0 {
			continue
		}
		ls := strings.Fields(line)
		if strings.Compare(ls[0], "cpu") == 0 {
			ParseStatLine(line, -1, m.config.ExcludeMetrics, out)
		} else if strings.HasPrefix(ls[0], "cpu") {
			cpustr := strings.TrimLeft(ls[0], "cpu")
			cpu, _ := strconv.Atoi(cpustr)
			ParseStatLine(line, cpu, m.config.ExcludeMetrics, out)
		}
	}
}

func (m *CpustatCollector) Close() {
	m.init = false
	return
}
