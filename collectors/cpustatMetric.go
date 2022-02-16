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
	config   CpustatCollectorConfig
	matches  map[string]int
	cputags  map[string]map[string]string
	nodetags map[string]string
}

func (m *CpustatCollector) Init(config json.RawMessage) error {
	m.name = "CpustatCollector"
	m.setup()
	m.meta = map[string]string{"source": m.name, "group": "CPU"}
	m.nodetags = map[string]string{"type": "node"}
	if len(config) > 0 {
		err := json.Unmarshal(config, &m.config)
		if err != nil {
			return err
		}
	}
	matches := []string{
		"",
		"cpu_user",
		"cpu_nice",
		"cpu_system",
		"cpu_idle",
		"cpu_iowait",
		"cpu_irq",
		"cpu_softirq",
		"cpu_steal",
		"cpu_guest",
		"cpu_guest_nice",
	}

	m.matches = make(map[string]int)
	for index, match := range matches {
		if len(match) > 0 {
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
	}

	// Check input file
	file, err := os.Open(string(CPUSTATFILE))
	if err != nil {
		cclog.ComponentError(m.name, err.Error())
	}
	defer file.Close()

	// Pre-generate tags for all CPUs
	m.cputags = make(map[string]map[string]string)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		linefields := strings.Fields(line)
		if strings.HasPrefix(linefields[0], "cpu") && strings.Compare(linefields[0], "cpu") != 0 {
			cpustr := strings.TrimLeft(linefields[0], "cpu")
			cpu, _ := strconv.Atoi(cpustr)
			m.cputags[linefields[0]] = map[string]string{"type": "cpu", "type-id": fmt.Sprintf("%d", cpu)}
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
	for name, value := range values {
		y, err := lp.New(name, tags, m.meta, map[string]interface{}{"value": (value * 100.0) / total}, time.Now())
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

	y, err := lp.New("num_cpus",
		m.nodetags,
		m.meta,
		map[string]interface{}{"value": int(num_cpus)},
		time.Now(),
	)
	if err == nil {
		output <- y
	}
}

func (m *CpustatCollector) Close() {
	m.init = false
}
