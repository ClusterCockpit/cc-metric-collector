package collectors

/*
#cgo CFLAGS: -I./likwid
#cgo LDFLAGS: -L./likwid -llikwid -llikwid-hwloc -lm -Wl,--unresolved-symbols=ignore-in-object-files
#include <stdlib.h>
#include <likwid.h>
*/
import "C"

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unsafe"

	cclog "github.com/ClusterCockpit/cc-metric-collector/internal/ccLogger"
	lp "github.com/ClusterCockpit/cc-metric-collector/internal/ccMetric"
	topo "github.com/ClusterCockpit/cc-metric-collector/internal/ccTopology"
	agg "github.com/ClusterCockpit/cc-metric-collector/internal/metricAggregator"
	"github.com/NVIDIA/go-nvml/pkg/dl"
)

type MetricScope string

const (
	METRIC_SCOPE_HWTHREAD = iota
	METRIC_SCOPE_CORE
	METRIC_SCOPE_LLC
	METRIC_SCOPE_NUMA
	METRIC_SCOPE_DIE
	METRIC_SCOPE_SOCKET
	METRIC_SCOPE_NODE
)

func (ms MetricScope) String() string {
	return string(ms)
}

func (ms MetricScope) Likwid() string {
	LikwidDomains := map[string]string{
		"cpu":        "",
		"core":       "",
		"llc":        "C",
		"numadomain": "M",
		"die":        "D",
		"socket":     "S",
		"node":       "N",
	}
	return LikwidDomains[string(ms)]
}

func (ms MetricScope) Granularity() int {
	for i, g := range GetAllMetricScopes() {
		if ms == g {
			return i
		}
	}
	return -1
}

func GetAllMetricScopes() []MetricScope {
	return []MetricScope{"cpu" /*, "core", "llc", "numadomain", "die",*/, "socket", "node"}
}

const (
	LIKWID_LIB_NAME     = "liblikwid.so"
	LIKWID_LIB_DL_FLAGS = dl.RTLD_LAZY | dl.RTLD_GLOBAL
)

type LikwidCollectorMetricConfig struct {
	Name string `json:"name"` // Name of the metric
	Calc string `json:"calc"` // Calculation for the metric using
	//Aggr        string      `json:"aggregation"` // if scope unequal to LIKWID metric scope, the values are combined (sum, min, max, mean or avg, median)
	Scope       MetricScope `json:"scope"` // scope for calculation. subscopes are aggregated using the 'aggregation' function
	Publish     bool        `json:"publish"`
	granulatity MetricScope
}

type LikwidCollectorEventsetConfig struct {
	Events      map[string]string `json:"events"`
	granulatity map[string]MetricScope
	Metrics     []LikwidCollectorMetricConfig `json:"metrics"`
}

type LikwidCollectorConfig struct {
	Eventsets      []LikwidCollectorEventsetConfig `json:"eventsets"`
	Metrics        []LikwidCollectorMetricConfig   `json:"globalmetrics,omitempty"`
	ForceOverwrite bool                            `json:"force_overwrite,omitempty"`
	InvalidToZero  bool                            `json:"invalid_to_zero,omitempty"`
}

type LikwidCollector struct {
	metricCollector
	cpulist       []C.int
	cpu2tid       map[int]int
	sock2tid      map[int]int
	scopeRespTids map[MetricScope]map[int]int
	metrics       map[C.int]map[string]int
	groups        []C.int
	config        LikwidCollectorConfig
	results       map[int]map[int]map[string]interface{}
	mresults      map[int]map[int]map[string]float64
	gmresults     map[int]map[string]float64
	basefreq      float64
	running       bool
}

type LikwidMetric struct {
	name      string
	search    string
	scope     MetricScope
	group_idx int
}

func eventsToEventStr(events map[string]string) string {
	elist := make([]string, 0)
	for k, v := range events {
		elist = append(elist, fmt.Sprintf("%s:%s", v, k))
	}
	return strings.Join(elist, ",")
}

func getGranularity(counter, event string) MetricScope {
	if strings.HasPrefix(counter, "PMC") || strings.HasPrefix(counter, "FIXC") {
		return "cpu"
	} else if strings.Contains(counter, "BOX") || strings.Contains(counter, "DEV") {
		return "socket"
	} else if strings.HasPrefix(counter, "PWR") {
		if event == "RAPL_CORE_ENERGY" {
			return "cpu"
		} else {
			return "socket"
		}
	}
	return "unknown"
}

func getBaseFreq() float64 {
	var freq float64 = math.NaN()
	C.power_init(0)
	info := C.get_powerInfo()
	if float64(info.baseFrequency) != 0 {
		freq = float64(info.baseFrequency) * 1e3
	} else {
		buffer, err := ioutil.ReadFile("/sys/devices/system/cpu/cpu0/cpufreq/bios_limit")
		if err == nil {
			data := strings.Replace(string(buffer), "\n", "", -1)
			x, err := strconv.ParseInt(data, 0, 64)
			if err == nil {
				freq = float64(x) * 1e3
			}
		}
	}
	return freq
}

func (m *LikwidCollector) initGranularity() {
	splitRegex := regexp.MustCompile("[+-/*()]")
	for _, evset := range m.config.Eventsets {
		evset.granulatity = make(map[string]MetricScope)
		for counter, event := range evset.Events {
			gran := getGranularity(counter, event)
			if gran.Granularity() >= 0 {
				evset.granulatity[counter] = gran
			}
		}
		for i, metric := range evset.Metrics {
			s := splitRegex.Split(metric.Calc, -1)
			gran := MetricScope("cpu")
			evset.Metrics[i].granulatity = gran
			for _, x := range s {
				if _, ok := evset.Events[x]; ok {
					if evset.granulatity[x].Granularity() > gran.Granularity() {
						gran = evset.granulatity[x]
					}
				}
			}
			evset.Metrics[i].granulatity = gran
		}
	}
	for i, metric := range m.config.Metrics {
		s := splitRegex.Split(metric.Calc, -1)
		gran := MetricScope("cpu")
		m.config.Metrics[i].granulatity = gran
		for _, x := range s {
			for _, evset := range m.config.Eventsets {
				for _, m := range evset.Metrics {
					if m.Name == x && m.granulatity.Granularity() > gran.Granularity() {
						gran = m.granulatity
					}
				}
			}
		}
		m.config.Metrics[i].granulatity = gran
	}
}

type TopoResolveFunc func(cpuid int) int

func (m *LikwidCollector) getResponsiblities() map[MetricScope]map[int]int {
	get_cpus := func(scope MetricScope) map[int]int {
		var slist []int
		var cpu C.int
		var input func(index int) string
		switch scope {
		case "node":
			slist = []int{0}
			input = func(index int) string { return "N:0" }
		case "socket":
			input = func(index int) string { return fmt.Sprintf("%s%d:0", scope.Likwid(), index) }
			slist = topo.SocketList()
		// case "numadomain":
		// 	input = func(index int) string { return fmt.Sprintf("%s%d:0", scope.Likwid(), index) }
		// 	slist = topo.NumaNodeList()
		// 	cclog.Debug(scope, " ", input(0), " ", slist)
		// case "die":
		// 	input = func(index int) string { return fmt.Sprintf("%s%d:0", scope.Likwid(), index) }
		// 	slist = topo.DieList()
		// case "llc":
		// 	input = fmt.Sprintf("%s%d:0", scope.Likwid(), s)
		// 	slist = topo.LLCacheList()
		case "cpu":
			input = func(index int) string { return fmt.Sprintf("%d", index) }
			slist = topo.CpuList()
		case "hwthread":
			input = func(index int) string { return fmt.Sprintf("%d", index) }
			slist = topo.CpuList()
		}
		outmap := make(map[int]int)
		for _, s := range slist {
			t := C.CString(input(s))
			clen := C.cpustr_to_cpulist(t, &cpu, 1)
			if int(clen) == 1 {
				outmap[s] = m.cpu2tid[int(cpu)]
			} else {
				cclog.Error(fmt.Sprintf("Cannot determine responsible CPU for %s", input(s)))
				outmap[s] = -1
			}
			C.free(unsafe.Pointer(t))
		}
		return outmap
	}

	scopes := GetAllMetricScopes()
	complete := make(map[MetricScope]map[int]int)
	for _, s := range scopes {
		complete[s] = get_cpus(s)
	}
	return complete
}

func (m *LikwidCollector) Init(config json.RawMessage) error {
	var ret C.int
	m.name = "LikwidCollector"
	if len(config) > 0 {
		err := json.Unmarshal(config, &m.config)
		if err != nil {
			return err
		}
	}
	lib := dl.New(LIKWID_LIB_NAME, LIKWID_LIB_DL_FLAGS)
	if lib == nil {
		return fmt.Errorf("error instantiating DynamicLibrary for %s", LIKWID_LIB_NAME)
	}
	if m.config.ForceOverwrite {
		cclog.ComponentDebug(m.name, "Set LIKWID_FORCE=1")
		os.Setenv("LIKWID_FORCE", "1")
	}
	m.setup()

	m.meta = map[string]string{"source": m.name, "group": "PerfCounter"}
	cclog.ComponentDebug(m.name, "Get cpulist and init maps and lists")
	cpulist := topo.CpuList()
	m.cpulist = make([]C.int, len(cpulist))
	m.cpu2tid = make(map[int]int)
	for i, c := range cpulist {
		m.cpulist[i] = C.int(c)
		m.cpu2tid[c] = i

	}
	m.results = make(map[int]map[int]map[string]interface{})
	m.mresults = make(map[int]map[int]map[string]float64)
	m.gmresults = make(map[int]map[string]float64)
	cclog.ComponentDebug(m.name, "initialize LIKWID topology")
	ret = C.topology_init()
	if ret != 0 {
		err := errors.New("failed to initialize LIKWID topology")
		cclog.ComponentError(m.name, err.Error())
		return err
	}

	// Determine which counter works at which level. PMC*: cpu, *BOX*: socket, ...
	m.initGranularity()
	// Generate map for MetricScope -> scope_id (like socket id) -> responsible id (offset in cpulist)
	m.scopeRespTids = m.getResponsiblities()

	cclog.ComponentDebug(m.name, "initialize LIKWID perfmon module")
	ret = C.perfmon_init(C.int(len(m.cpulist)), &m.cpulist[0])
	if ret != 0 {
		C.topology_finalize()
		err := errors.New("failed to initialize LIKWID topology")
		cclog.ComponentError(m.name, err.Error())
		return err
	}

	// This is for the global metrics computation test
	globalParams := make(map[string]interface{})
	globalParams["time"] = float64(1.0)
	globalParams["inverseClock"] = float64(1.0)
	// While adding the events, we test the metrics whether they can be computed at all
	for i, evset := range m.config.Eventsets {
		estr := eventsToEventStr(evset.Events)
		// Generate parameter list for the metric computing test
		params := make(map[string]interface{})
		params["time"] = float64(1.0)
		params["inverseClock"] = float64(1.0)
		for counter := range evset.Events {
			params[counter] = float64(1.0)
		}
		for _, metric := range evset.Metrics {
			// Try to evaluate the metric
			_, err := agg.EvalFloat64Condition(metric.Calc, params)
			if err != nil {
				cclog.ComponentError(m.name, "Calculation for metric", metric.Name, "failed:", err.Error())
				continue
			}
			// If the metric is not in the parameter list for the global metrics, add it
			if _, ok := globalParams[metric.Name]; !ok {
				globalParams[metric.Name] = float64(1.0)
			}
		}
		// Now we add the list of events to likwid
		cstr := C.CString(estr)
		gid := C.perfmon_addEventSet(cstr)
		if gid >= 0 {
			m.groups = append(m.groups, gid)
		}
		C.free(unsafe.Pointer(cstr))
		m.results[i] = make(map[int]map[string]interface{})
		m.mresults[i] = make(map[int]map[string]float64)
		for tid := range m.cpulist {
			m.results[i][tid] = make(map[string]interface{})
			m.mresults[i][tid] = make(map[string]float64)
			if i == 0 {
				m.gmresults[tid] = make(map[string]float64)
			}
		}
	}
	for _, metric := range m.config.Metrics {
		// Try to evaluate the global metric
		_, err := agg.EvalFloat64Condition(metric.Calc, globalParams)
		if err != nil {
			cclog.ComponentError(m.name, "Calculation for metric", metric.Name, "failed:", err.Error())
			continue
		}
	}

	// If no event set could be added, shut down LikwidCollector
	if len(m.groups) == 0 {
		C.perfmon_finalize()
		C.topology_finalize()
		err := errors.New("no LIKWID performance group initialized")
		cclog.ComponentError(m.name, err.Error())
		return err
	}
	m.basefreq = getBaseFreq()
	cclog.ComponentDebug(m.name, "BaseFreq", m.basefreq)
	m.init = true
	return nil
}

// take a measurement for 'interval' seconds of event set index 'group'
func (m *LikwidCollector) takeMeasurement(group int, interval time.Duration) error {
	var ret C.int
	gid := m.groups[group]
	ret = C.perfmon_setupCounters(gid)
	if ret != 0 {
		gctr := C.GoString(C.perfmon_getGroupName(gid))
		err := fmt.Errorf("failed to setup performance group %d (%s)", gid, gctr)
		return err
	}
	ret = C.perfmon_startCounters()
	if ret != 0 {
		gctr := C.GoString(C.perfmon_getGroupName(gid))
		err := fmt.Errorf("failed to start performance group %d (%s)", gid, gctr)
		return err
	}
	m.running = true
	time.Sleep(interval)
	m.running = false
	ret = C.perfmon_stopCounters()
	if ret != 0 {
		gctr := C.GoString(C.perfmon_getGroupName(gid))
		err := fmt.Errorf("failed to stop performance group %d (%s)", gid, gctr)
		return err
	}
	return nil
}

// Get all measurement results for an event set, derive the metric values out of the measurement results and send it
func (m *LikwidCollector) calcEventsetMetrics(group int, interval time.Duration, output chan lp.CCMetric) error {
	var eidx C.int
	evset := m.config.Eventsets[group]
	gid := m.groups[group]
	invClock := float64(1.0 / m.basefreq)

	// Go over events and get the results
	for eidx = 0; int(eidx) < len(evset.Events); eidx++ {
		ctr := C.perfmon_getCounterName(gid, eidx)
		ev := C.perfmon_getEventName(gid, eidx)
		gctr := C.GoString(ctr)
		gev := C.GoString(ev)
		// MetricScope for the counter (and if needed the event)
		scope := getGranularity(gctr, gev)
		// Get the map scope-id -> tids
		// This way we read less counters like only the responsible hardware thread for a socket
		scopemap := m.scopeRespTids[scope]
		for _, tid := range scopemap {
			if tid >= 0 {
				m.results[group][tid]["time"] = interval.Seconds()
				m.results[group][tid]["inverseClock"] = invClock
				res := C.perfmon_getLastResult(gid, eidx, C.int(tid))
				m.results[group][tid][gctr] = float64(res)
			}
		}
	}

	// Go over the event set metrics, derive the value out of the event:counter values and send it
	for _, metric := range evset.Metrics {
		// The metric scope is determined in the Init() function
		// Get the map scope-id -> tids
		scopemap := m.scopeRespTids[metric.Scope]
		for domain, tid := range scopemap {
			if tid >= 0 {
				value, err := agg.EvalFloat64Condition(metric.Calc, m.results[group][tid])
				if err != nil {
					cclog.ComponentError(m.name, "Calculation for metric", metric.Name, "failed:", err.Error())
					continue
				}
				m.mresults[group][tid][metric.Name] = value
				if m.config.InvalidToZero && math.IsNaN(value) {
					value = 0.0
				}
				if m.config.InvalidToZero && math.IsInf(value, 0) {
					value = 0.0
				}
				// Now we have the result, send it with the proper tags
				if !math.IsNaN(value) {
					if metric.Publish {
						tags := map[string]string{"type": metric.Scope.String()}
						if metric.Scope != "node" {
							tags["type-id"] = fmt.Sprintf("%d", domain)
						}
						fields := map[string]interface{}{"value": value}
						y, err := lp.New(metric.Name, tags, m.meta, fields, time.Now())
						if err == nil {
							output <- y
						}
					}
				}
			}
		}
	}

	return nil
}

// Go over the global metrics, derive the value out of the event sets' metric values and send it
func (m *LikwidCollector) calcGlobalMetrics(interval time.Duration, output chan lp.CCMetric) error {
	for _, metric := range m.config.Metrics {
		scopemap := m.scopeRespTids[metric.Scope]
		for domain, tid := range scopemap {
			if tid >= 0 {
				// Here we generate parameter list
				params := make(map[string]interface{})
				for j := range m.groups {
					for mname, mres := range m.mresults[j][tid] {
						params[mname] = mres
					}
				}
				// Evaluate the metric
				value, err := agg.EvalFloat64Condition(metric.Calc, params)
				if err != nil {
					cclog.ComponentError(m.name, "Calculation for metric", metric.Name, "failed:", err.Error())
					continue
				}
				m.gmresults[tid][metric.Name] = value
				if m.config.InvalidToZero && math.IsNaN(value) {
					value = 0.0
				}
				if m.config.InvalidToZero && math.IsInf(value, 0) {
					value = 0.0
				}
				// Now we have the result, send it with the proper tags
				if !math.IsNaN(value) {
					if metric.Publish {
						tags := map[string]string{"type": metric.Scope.String()}
						if metric.Scope != "node" {
							tags["type-id"] = fmt.Sprintf("%d", domain)
						}
						fields := map[string]interface{}{"value": value}
						y, err := lp.New(metric.Name, tags, m.meta, fields, time.Now())
						if err == nil {
							output <- y
						}
					}
				}
			}
		}
	}
	return nil
}

// main read function taking multiple measurement rounds, each 'interval' seconds long
func (m *LikwidCollector) Read(interval time.Duration, output chan lp.CCMetric) {
	if !m.init {
		return
	}

	for i := range m.groups {
		// measure event set 'i' for 'interval' seconds
		err := m.takeMeasurement(i, interval)
		if err != nil {
			cclog.ComponentError(m.name, err.Error())
			return
		}
		// read measurements and derive event set metrics
		m.calcEventsetMetrics(i, interval, output)
	}
	// use the event set metrics to derive the global metrics
	m.calcGlobalMetrics(interval, output)
}

func (m *LikwidCollector) Close() {
	if m.init {
		cclog.ComponentDebug(m.name, "Closing ...")
		m.init = false
		if m.running {
			cclog.ComponentDebug(m.name, "Stopping counters")
			C.perfmon_stopCounters()
		}
		cclog.ComponentDebug(m.name, "Finalize LIKWID perfmon module")
		C.perfmon_finalize()
		cclog.ComponentDebug(m.name, "Finalize LIKWID topology module")
		C.topology_finalize()
		cclog.ComponentDebug(m.name, "Closing done")
	}
}
