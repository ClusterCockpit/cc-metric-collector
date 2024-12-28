package collectors

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	lp "github.com/ClusterCockpit/cc-energy-manager/pkg/cc-message"
	cclog "github.com/ClusterCockpit/cc-metric-collector/pkg/ccLogger"
	topo "github.com/ClusterCockpit/cc-metric-collector/pkg/ccTopology"
)

var perf_number_regex = regexp.MustCompile(`(\d+),(\d+)`)

const PERF_NOT_COUNTED = "<not counted>"
const PERF_UNIT_NULL = "(null)"

var VALID_METRIC_TYPES = []string{
	"hwthread",
	"core",
	"llc",
	"socket",
	"die",
	"node",
	"memoryDomain",
}

type PerfCmdCollectorEventConfig struct {
	Metric      string            `json:"metric"`                     // metric name
	Event       string            `json:"event"`                      // perf event configuration
	Type        string            `json:"type"`                       // Metric type (aka node, socket, hwthread, ...)
	Tags        map[string]string `json:"tags,omitempty"`             // extra tags for the metric
	Meta        map[string]string `json:"meta,omitempty"`             // extra meta information for the metric
	Unit        string            `json:"unit,omitempty"`             // unit of metric (if any)
	UsePerfUnit bool              `json:"use_perf_unit,omitempty"`    // for some events perf tells a metric
	TypeAgg     string            `json:"type_aggregation,omitempty"` // how to aggregate cpu-data to metric type
	Publish     bool              `json:"publish,omitempty"`
	//lastCounterValue float64
	//lastMetricValue  float64
	collectorTags *map[string]string
	collectorMeta *map[string]string
	useCpus       map[int][]int
}

type PerfCmdCollectorExpression struct {
	Metric     string `json:"metric"`                     // metric name
	Expression string `json:"expression"`                 // expression based on metrics
	Type       string `json:"type"`                       // Metric type (aka node, socket, hwthread, ...)
	TypeAgg    string `json:"type_aggregation,omitempty"` // how to aggregate cpu-data to metric type
	Publish    bool   `json:"publish,omitempty"`
}

// These are the fields we read from the JSON configuration
type PerfCmdCollectorConfig struct {
	Metrics     []PerfCmdCollectorEventConfig `json:"metrics"`
	Expressions []PerfCmdCollectorExpression  `json:"expressions"`
	PerfCmd     string                        `json:"perf_command,omitempty"`
}

// This contains all variables we need during execution and the variables
// defined by metricCollector (name, init, ...)
type PerfCmdCollector struct {
	metricCollector
	config          PerfCmdCollectorConfig                  // the configuration structure
	meta            map[string]string                       // default meta information
	tags            map[string]string                       // default tags
	metrics         map[string]*PerfCmdCollectorEventConfig // list of events for internal data
	perfEventString string
}

// Functions to implement MetricCollector interface
// Init(...), Read(...), Close()
// See: metricCollector.go

// Init initializes the sample collector
// Called once by the collector manager
// All tags, meta data tags and metrics that do not change over the runtime should be set here
func (m *PerfCmdCollector) Init(config json.RawMessage) error {
	var err error = nil
	// Always set the name early in Init() to use it in cclog.Component* functions
	m.name = "PerfCmdCollector"
	m.parallel = false
	// This is for later use, also call it early
	m.setup()
	// Tell whether the collector should be run in parallel with others (reading files, ...)
	// or it should be run serially, mostly for collectors actually doing measurements
	// because they should not measure the execution of the other collectors
	m.parallel = true
	// Define meta information sent with each metric
	// (Can also be dynamic or this is the basic set with extension through AddMeta())
	m.meta = map[string]string{"source": m.name, "group": "PerfCounter"}
	// Define tags sent with each metric
	// The 'type' tag is always needed, it defines the granularity of the metric
	// node -> whole system
	// socket -> CPU socket (requires socket ID as 'type-id' tag)
	// die -> CPU die (requires CPU die ID as 'type-id' tag)
	// memoryDomain -> NUMA domain (requires NUMA domain ID as 'type-id' tag)
	// llc -> Last level cache (requires last level cache ID as 'type-id' tag)
	// core -> single CPU core that may consist of multiple hardware threads (SMT) (requires core ID as 'type-id' tag)
	// hwthtread -> single CPU hardware thread (requires hardware thread ID as 'type-id' tag)
	// accelerator -> A accelerator device like GPU or FPGA (requires an accelerator ID as 'type-id' tag)
	m.tags = map[string]string{"type": "node"}
	// Read in the JSON configuration
	if len(config) > 0 {
		err = json.Unmarshal(config, &m.config)
		if err != nil {
			cclog.ComponentError(m.name, "Error reading config:", err.Error())
			return err
		}
	}
	m.config.PerfCmd = "perf"
	if len(m.config.PerfCmd) > 0 {
		_, err := os.Stat(m.config.PerfCmd)
		if err != nil {
			abs, err := exec.LookPath(m.config.PerfCmd)
			if err != nil {
				cclog.ComponentError(m.name, "Error looking up perf command", m.config.PerfCmd, ":", err.Error())
				return err
			}
			m.config.PerfCmd = abs
		}
	}

	// Set up everything that the collector requires during the Read() execution
	// Check files required, test execution of some commands, create data structure
	// for all topological entities (sockets, NUMA domains, ...)
	// Return some useful error message in case of any failures

	valid_metrics := make([]*PerfCmdCollectorEventConfig, 0)
	valid_events := make([]string, 0)
	test_type := func(Type string) bool {
		for _, t := range VALID_METRIC_TYPES {
			if Type == t {
				return true
			}
		}
		return false
	}
	for i, metric := range m.config.Metrics {
		if !test_type(metric.Type) {
			cclog.ComponentError(m.name, "Metric", metric.Metric, "has an invalid type")
			continue
		}
		cmd := exec.Command(m.config.PerfCmd, "stat", "--null", "-e", metric.Event, "hostname")
		cclog.ComponentDebug(m.name, "Running", cmd.String())
		err := cmd.Run()
		if err != nil {
			cclog.ComponentError(m.name, "Event", metric.Event, "not available in perf", err.Error())
		} else {
			valid_metrics = append(valid_metrics, &m.config.Metrics[i])
		}
	}
	if len(valid_metrics) == 0 {
		return errors.New("no configured metric available through perf")
	}

	IntToStringList := func(ilist []int) []string {
		list := make([]string, 0)
		for _, i := range ilist {
			list = append(list, fmt.Sprintf("%v", i))
		}
		return list
	}

	m.metrics = make(map[string]*PerfCmdCollectorEventConfig, 0)
	for _, metric := range valid_metrics {
		metric.collectorMeta = &m.meta
		metric.collectorTags = &m.tags
		metric.useCpus = make(map[int][]int)
		tlist := topo.GetTypeList(metric.Type)
		cclog.ComponentDebug(m.name, "Metric", metric.Metric, "with type", metric.Type, ":", strings.Join(IntToStringList(tlist), ","))

		for _, t := range tlist {
			metric.useCpus[t] = topo.GetTypeHwthreads(metric.Type, t)
			cclog.ComponentDebug(m.name, "Metric", metric.Metric, "with type", metric.Type, "and ID", t, ":", strings.Join(IntToStringList(metric.useCpus[t]), ","))
		}

		m.metrics[metric.Event] = metric
		valid_events = append(valid_events, metric.Event)
	}
	m.perfEventString = strings.Join(valid_events, ",")
	cclog.ComponentDebug(m.name, "perfEventString", m.perfEventString)

	// Set this flag only if everything is initialized properly, all required files exist, ...
	m.init = true
	return err
}

type PerfEventJson struct {
	CounterValue string `json:"counter-value"`
	counterValue float64
	MetricValue  string `json:"metric-value"`
	metricValue  float64
	CounterUnit  string `json:"unit"`
	counterUnit  string
	MetricUnit   string `json:"metric-unit"`
	metricUnit   string
	Cpu          string `json:"cpu,omitempty"`
	cpu          int
	Event        string  `json:"event"`
	Runtime      uint64  `json:"event-runtime"`
	PcntRunning  float64 `json:"pcnt-running"`
	metrictypeid string
	metrictype   string
	metricname   string
	publish      bool
}

func parseEvent(line string) (*PerfEventJson, error) {
	data := PerfEventJson{}

	tmp := perf_number_regex.ReplaceAllString(line, `$1.$2`)
	err := json.Unmarshal([]byte(tmp), &data)
	if err != nil {
		return nil, err
	}
	if len(data.CounterValue) > 0 && data.CounterValue != PERF_NOT_COUNTED {
		val, err := strconv.ParseFloat(data.CounterValue, 64)
		if err == nil {
			if data.PcntRunning != 100.0 {
				val = (val / data.PcntRunning) * 100
			}
			data.counterValue = val
		}
	}
	if len(data.MetricValue) > 0 && data.MetricValue != PERF_NOT_COUNTED {
		val, err := strconv.ParseFloat(data.MetricValue, 64)
		if err == nil {
			if data.PcntRunning != 100.0 {
				val = (val / data.PcntRunning) * 100
			}
			data.metricValue = val
		}
	}
	if len(data.CounterUnit) > 0 && data.CounterUnit != PERF_UNIT_NULL {
		data.counterUnit = data.CounterUnit
	}
	if len(data.MetricUnit) > 0 && data.MetricUnit != PERF_UNIT_NULL {
		data.metricUnit = data.MetricUnit
	}
	if len(data.Cpu) > 0 {
		val, err := strconv.ParseInt(data.Cpu, 10, 64)
		if err == nil {
			data.cpu = int(val)
		}
	}

	return &data, nil
}

func perfdataToMetric(data *PerfEventJson, config *PerfCmdCollectorEventConfig, timestamp time.Time) (lp.CCMetric, error) {
	metric, err := lp.NewMetric(config.Metric, *config.collectorTags, *config.collectorMeta, data.counterValue, timestamp)
	if err == nil {
		metric.AddTag("type", data.metrictype)
		if data.metrictype != "node" {
			metric.AddTag("type-id", data.metrictypeid)
		}
		for k, v := range config.Tags {
			metric.AddTag(k, v)
		}
		for k, v := range config.Meta {
			metric.AddMeta(k, v)
		}
		if len(config.Unit) > 0 {
			metric.AddMeta("unit", config.Unit)
		}
		if config.UsePerfUnit && (!metric.HasMeta("unit")) && (!metric.HasTag("unit")) {
			var unit string = ""
			if len(data.counterUnit) > 0 {
				unit = data.counterUnit
			} else if len(data.metricUnit) > 0 {
				unit = data.metricUnit
			}
			if len(unit) > 0 {
				metric.AddMeta("unit", unit)
			}
		}
		return metric, nil
	}
	return nil, err
}

// Read collects all metrics belonging to the sample collector
// and sends them through the output channel to the collector manager
func (m *PerfCmdCollector) Read(interval time.Duration, output chan lp.CCMessage) {
	perfdata := make([]*PerfEventJson, 0)
	// Create a sample metric
	timestamp := time.Now()

	cmd := exec.Command(m.config.PerfCmd, "stat", "-A", "-a", "-j", "-e", m.perfEventString, "/usr/bin/sleep", fmt.Sprintf("%d", int(interval.Seconds())))

	cclog.ComponentDebug(m.name, "Running", cmd.String())
	out, err := cmd.CombinedOutput()
	if err == nil {
		sout := strings.TrimSpace(string(out))
		for _, l := range strings.Split(sout, "\n") {
			d, err := parseEvent(l)
			if err == nil {
				perfdata = append(perfdata, d)
			}
		}
	} else {
		cclog.ComponentError(m.name, "Execution of", cmd.String(), "failed with", err.Error())
	}

	metricData := make([]*PerfEventJson, 0)
	for _, metricTmp := range m.config.Metrics {
		metricConfig := m.metrics[metricTmp.Event]
		for t, clist := range metricConfig.useCpus {
			val := float64(0)
			sum := float64(0)
			min := math.MaxFloat64
			max := float64(0)
			count := 0
			cunit := ""
			munit := ""
			for _, c := range clist {
				for _, d := range perfdata {
					if strings.HasPrefix(d.Event, metricConfig.Event) && d.cpu == c {
						//cclog.ComponentDebug(m.name, "do calc on CPU", c, ":", d.counterValue)
						sum += d.counterValue
						if d.counterValue < min {
							min = d.counterValue
						}
						if d.counterValue > max {
							max = d.counterValue
						}
						count++
						cunit = d.counterUnit
						munit = d.metricUnit
					}
				}
			}
			if metricConfig.TypeAgg == "sum" {
				val = sum
			} else if metricConfig.TypeAgg == "min" {
				val = min
			} else if metricConfig.TypeAgg == "max" {
				val = max
			} else if metricConfig.TypeAgg == "avg" || metricConfig.TypeAgg == "mean" {
				val = sum / float64(count)
			} else {
				val = sum
			}
			//cclog.ComponentDebug(m.name, "Metric", metricConfig.Metric, "type", metricConfig.Type, "ID", t, ":", val)
			metricData = append(metricData, &PerfEventJson{
				Event:        metricConfig.Event,
				metricname:   metricConfig.Metric,
				metrictype:   metricConfig.Type,
				metrictypeid: fmt.Sprintf("%v", t),
				counterValue: val,
				metricValue:  0,
				metricUnit:   munit,
				counterUnit:  cunit,
				publish:      metricConfig.Publish,
			})
		}

	}

	for _, d := range metricData {
		if d.publish {
			m, err := perfdataToMetric(d, m.metrics[d.Event], timestamp)
			if err == nil {
				output <- m
			}
		}
	}

}

// Close metric collector: close network connection, close files, close libraries, ...
// Called once by the collector manager
func (m *PerfCmdCollector) Close() {
	// Unset flag
	m.init = false
}
