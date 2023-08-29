package collectors

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	cclog "github.com/ClusterCockpit/cc-metric-collector/pkg/ccLogger"
	lp "github.com/ClusterCockpit/cc-metric-collector/pkg/ccMetric"
	sysconf "github.com/tklauser/go-sysconf"
)

const CPUSTATFILE = `/proc/stat`

type CpustatCollectorConfig struct {
	ExcludeMetrics []string `json:"exclude_metrics,omitempty"`
}

type CpustatCollector struct {
	metricCollector
	config        CpustatCollectorConfig
	lastTimestamp time.Time // Store time stamp of last tick to derive values
	matches       map[string]int
	cputags       map[string]map[string]string
	nodetags      map[string]string
	olddata       map[string]map[string]int64
}

func (m *CpustatCollector) Init(config json.RawMessage) error {
	m.name = "CpustatCollector"
	m.setup()
	m.parallel = true
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
	m.olddata = make(map[string]map[string]int64)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		linefields := strings.Fields(line)
		if strings.Compare(linefields[0], "cpu") == 0 {
			m.olddata["cpu"] = make(map[string]int64)
			for k, v := range m.matches {
				m.olddata["cpu"][k], _ = strconv.ParseInt(linefields[v], 0, 64)
			}
		} else if strings.HasPrefix(linefields[0], "cpu") && strings.Compare(linefields[0], "cpu") != 0 {
			cpustr := strings.TrimLeft(linefields[0], "cpu")
			cpu, _ := strconv.Atoi(cpustr)
			m.cputags[linefields[0]] = map[string]string{"type": "hwthread", "type-id": fmt.Sprintf("%d", cpu)}
			m.olddata[linefields[0]] = make(map[string]int64)
			for k, v := range m.matches {
				m.olddata[linefields[0]][k], _ = strconv.ParseInt(linefields[v], 0, 64)
			}
			num_cpus++
		}
	}
	m.lastTimestamp = time.Now()
	m.init = true
	return nil
}

func (m *CpustatCollector) parseStatLine(linefields []string, tags map[string]string, output chan lp.CCMetric, now time.Time, tsdelta time.Duration) {
	values := make(map[string]float64)
	clktck, _ := sysconf.Sysconf(sysconf.SC_CLK_TCK)
	for match, index := range m.matches {
		if len(match) > 0 {
			x, err := strconv.ParseInt(linefields[index], 0, 64)
			if err == nil {
				vdiff := x - m.olddata[linefields[0]][match]
				m.olddata[linefields[0]][match] = x // Store new value for next run
				values[match] = float64(vdiff) / float64(tsdelta.Seconds()) / float64(clktck)
			}
		}
	}

	sum := float64(0)
	for name, value := range values {
		sum += value
		y, err := lp.New(name, tags, m.meta, map[string]interface{}{"value": value * 100}, now)
		if err == nil {
			output <- y
		}
	}
	if v, ok := values["cpu_idle"]; ok {
		sum -= v
		y, err := lp.New("cpu_used", tags, m.meta, map[string]interface{}{"value": sum * 100}, now)
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
	now := time.Now()
	tsdelta := now.Sub(m.lastTimestamp)

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
			m.parseStatLine(linefields, m.nodetags, output, now, tsdelta)
		} else if strings.HasPrefix(linefields[0], "cpu") {
			m.parseStatLine(linefields, m.cputags[linefields[0]], output, now, tsdelta)
			num_cpus++
		}
	}

	num_cpus_metric, err := lp.New("num_cpus",
		m.nodetags,
		m.meta,
		map[string]interface{}{"value": int(num_cpus)},
		now,
	)
	if err == nil {
		output <- num_cpus_metric
	}

	m.lastTimestamp = now
}

func (m *CpustatCollector) Close() {
	m.init = false
}
