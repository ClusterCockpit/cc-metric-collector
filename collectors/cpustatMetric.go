package collectors

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	cclog "github.com/ClusterCockpit/cc-metric-collector/internal/ccLogger"
	lp "github.com/ClusterCockpit/cc-metric-collector/internal/ccMetric"
)

const CPUSTATFILE = `/proc/stat`

type CpustatCollectorConfig struct {
	ExcludeMetrics []string `json:"exclude_metrics,omitempty"`
}

type CpustatCollector struct {
	metricCollector
	config          CpustatCollectorConfig
	matches         map[string]int
	cputags         map[string]map[string]string
	nodetags        map[string]string
	num_cpus_metric lp.CCMetric
}

func (m *CpustatCollector) Init(config json.RawMessage) error {
	m.name = "CpustatCollector"
	m.setup()
	m.meta = map[string]string{"source": m.name, "group": "CPU", "unit": "Percent"}
	m.nodetags = map[string]string{"type": "node"}
	if len(config) > 0 {
		err := json.Unmarshal(config, &m.config)
		if err != nil {
			return err
		}
	}
	matches := map[string]int{
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

	m.matches = make(map[string]int)
	for match, index := range matches {
		doExclude := false
		for _, exclude := range m.config.ExcludeMetrics {
			if match == exclude {
				doExclude = true
				break
			}
		}
		if !doExclude {
			m.matches[match] = index
		}
	}

	// Check input file
	file, err := os.Open(string(CPUSTATFILE))
	if err != nil {
		cclog.ComponentError(m.name, err.Error())
	}
	defer file.Close()

	// Pre-generate tags for all CPUs
	num_cpus := 0
	m.cputags = make(map[string]map[string]string)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		linefields := strings.Fields(line)
		if strings.HasPrefix(linefields[0], "cpu") && strings.Compare(linefields[0], "cpu") != 0 {
			cpustr := strings.TrimLeft(linefields[0], "cpu")
			cpu, _ := strconv.Atoi(cpustr)
			m.cputags[linefields[0]] = map[string]string{"type": "cpu", "type-id": fmt.Sprintf("%d", cpu)}
			num_cpus++
		}
	}
	m.init = true
	return nil
}

func (m *CpustatCollector) parseStatLine(linefields []string, tags map[string]string, output chan lp.CCMetric) {
	values := make(map[string]float64)
	total := 0.0
	for match, index := range m.matches {
		if len(match) > 0 {
			x, err := strconv.ParseInt(linefields[index], 0, 64)
			if err == nil {
				values[match] = float64(x)
				total += values[match]
			}
		}
	}
	t := time.Now()
	for name, value := range values {
		y, err := lp.New(name, tags, m.meta, map[string]interface{}{"value": (value * 100.0) / total}, t)
		if err == nil {
			output <- y
		}
	}
}

func (m *CpustatCollector) Read(interval time.Duration, output chan lp.CCMetric) {
	if !m.init {
		return
	}
	num_cpus := 0
	file, err := os.Open(string(CPUSTATFILE))
	if err != nil {
		cclog.ComponentError(m.name, err.Error())
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		linefields := strings.Fields(line)
		if strings.Compare(linefields[0], "cpu") == 0 {
			m.parseStatLine(linefields, m.nodetags, output)
		} else if strings.HasPrefix(linefields[0], "cpu") {
			m.parseStatLine(linefields, m.cputags[linefields[0]], output)
			num_cpus++
		}
	}

	num_cpus_metric, err := lp.New("num_cpus",
		m.nodetags,
		m.meta,
		map[string]interface{}{"value": int(num_cpus)},
		time.Now(),
	)
	if err == nil {
		output <- num_cpus_metric
	}
}

func (m *CpustatCollector) Close() {
	m.init = false
}
