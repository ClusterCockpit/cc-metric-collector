package collectors

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	lp "github.com/ClusterCockpit/cc-lib/ccMessage"
	cclog "github.com/ClusterCockpit/cc-metric-collector/pkg/ccLogger"
	sysconf "github.com/tklauser/go-sysconf"
)

const CPUSTATFILE = `/proc/stat`

// CpustatCollectorConfig for the cpustat collector.
type CpustatCollectorConfig struct {
	ExcludeMetrics []string `json:"exclude_metrics,omitempty"`
	OnlyMetrics    []string `json:"only_metrics,omitempty"`
}

type CpustatCollector struct {
	metricCollector
	config        CpustatCollectorConfig
	lastTimestamp time.Time
	matches       map[string]int
	cputags       map[string]map[string]string
	nodetags      map[string]string
	olddata       map[string]map[string]int64
}

func (m *CpustatCollector) Init(config json.RawMessage) error {
	m.name = "CpustatCollector"
	m.setup()
	m.parallel = true
	m.meta = map[string]string{"source": m.name, "group": "CPU"}
	m.nodetags = map[string]string{"type": "node"}

	if len(config) > 0 {
		if err := json.Unmarshal(config, &m.config); err != nil {
			return err
		}
	}

	// Define available metrics and their corresponding index in /proc/stat.
	metrics := map[string]int{
		"cpu_user":       1,
		"cpu_nice":       2,
		"cpu_system":     3,
		"cpu_idle":       4,
		"cpu_iowait":     5,
		"cpu_irq":        6,
		"cpu_softirq":    7,
		"cpu_steal":      8,
		"cpu_guest":      9,
		"cpu_guest_nice": 10,
	}
	m.matches = metrics

	// Open the file and initialize olddata and cputags.
	file, err := os.Open(CPUSTATFILE)
	if err != nil {
		cclog.ComponentError(m.name, err.Error())
	}
	defer file.Close()

	m.cputags = make(map[string]map[string]string)
	m.olddata = make(map[string]map[string]int64)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		linefields := strings.Fields(line)
		// Process the summary line "cpu" for node-level metrics.
		if strings.Compare(linefields[0], "cpu") == 0 {
			m.olddata["cpu"] = make(map[string]int64)
			for metric, index := range m.matches {
				m.olddata["cpu"][metric], _ = strconv.ParseInt(linefields[index], 0, 64)
			}
		} else if strings.HasPrefix(linefields[0], "cpu") && strings.Compare(linefields[0], "cpu") != 0 {
			// Process individual CPU lines for hardware thread metrics.
			cpustr := strings.TrimLeft(linefields[0], "cpu")
			cpu, _ := strconv.Atoi(cpustr)
			m.cputags[linefields[0]] = map[string]string{"type": "hwthread", "type-id": fmt.Sprintf("%d", cpu)}
			m.olddata[linefields[0]] = make(map[string]int64)
			for metric, index := range m.matches {
				m.olddata[linefields[0]][metric], _ = strconv.ParseInt(linefields[index], 0, 64)
			}
		}
	}
	m.lastTimestamp = time.Now()
	m.init = true
	return nil
}

func (m *CpustatCollector) shouldOutput(metricName string) bool {
	if len(m.config.OnlyMetrics) > 0 {
		for _, name := range m.config.OnlyMetrics {
			if name == metricName {
				return true
			}
		}
		return false
	}

	for _, ex := range m.config.ExcludeMetrics {
		if ex == metricName {
			return false
		}
	}
	return true
}

func (m *CpustatCollector) parseStatLine(linefields []string, tags map[string]string, output chan lp.CCMessage, now time.Time, tsdelta time.Duration) {
	clktck, _ := sysconf.Sysconf(sysconf.SC_CLK_TCK)
	for metric, index := range m.matches {
		currentVal, err := strconv.ParseInt(linefields[index], 0, 64)
		if err != nil {
			continue
		}
		// Calculate the delta since the last read.
		diff := currentVal - m.olddata[linefields[0]][metric]
		m.olddata[linefields[0]][metric] = currentVal
		// Calculate the percentage value.
		value := float64(diff) / tsdelta.Seconds() / float64(clktck) * 100
		if m.shouldOutput(metric) {
			msg, err := lp.NewMessage(metric, tags, m.meta, map[string]interface{}{"value": value}, now)
			if err == nil {
				msg.AddTag("unit", "Percent")
				output <- msg
			}
		}
	}

	// Compute and output 'cpu_used' as the sum of all metrics (excluding cpu_idle).
	sum := float64(0)
	for metric, index := range m.matches {
		if metric == "cpu_idle" {
			continue
		}
		currentVal, err := strconv.ParseInt(linefields[index], 0, 64)
		if err != nil {
			continue
		}
		diff := currentVal - m.olddata[linefields[0]][metric]
		sum += float64(diff) / tsdelta.Seconds() / float64(clktck)
	}
	if m.shouldOutput("cpu_used") {
		msg, err := lp.NewMessage("cpu_used", tags, m.meta, map[string]interface{}{"value": sum * 100}, now)
		if err == nil {
			msg.AddTag("unit", "Percent")
			output <- msg
		}
	}
}

func (m *CpustatCollector) Read(interval time.Duration, output chan lp.CCMessage) {
	if !m.init {
		return
	}
	now := time.Now()
	tsdelta := now.Sub(m.lastTimestamp)

	file, err := os.Open(CPUSTATFILE)
	if err != nil {
		cclog.ComponentError(m.name, err.Error())
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		linefields := strings.Fields(line)
		// Process node-level metrics.
		if strings.Compare(linefields[0], "cpu") == 0 {
			m.parseStatLine(linefields, m.nodetags, output, now, tsdelta)
		} else if strings.HasPrefix(linefields[0], "cpu") {
			// Process hardware thread metrics.
			m.parseStatLine(linefields, m.cputags[linefields[0]], output, now, tsdelta)
		}
	}
	// Output number of CPUs as a separate metric.
	numCPUs := len(m.cputags)
	if m.shouldOutput("num_cpus") {
		numCPUsMsg, err := lp.NewMessage("num_cpus",
			m.nodetags,
			m.meta,
			map[string]interface{}{"value": numCPUs},
			now,
		)
		if err == nil {
			output <- numCPUsMsg
		}
	}
	m.lastTimestamp = now
}

func (m *CpustatCollector) Close() {
	m.init = false
}
