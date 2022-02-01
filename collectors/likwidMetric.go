package collectors

/*
#cgo CFLAGS: -I./likwid
#cgo LDFLAGS: -L./likwid -llikwid -llikwid-hwloc -lm
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
	"github.com/PaesslerAG/gval"
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

func (ms MetricScope) Granularity() int {
	grans := []string{"hwthread", "core", "llc", "numadomain", "die", "socket", "node"}
	for i, g := range grans {
		if ms.String() == g {
			return i
		}
	}
	return -1
}

type LikwidCollectorMetricConfig struct {
	Name        string      `json:"name"`        // Name of the metric
	Calc        string      `json:"calc"`        // Calculation for the metric using
	Aggr        string      `json:"aggregation"` // if scope unequal to LIKWID metric scope, the values are combined (sum, min, max, mean or avg, median)
	Scope       MetricScope `json:"scope"`       // scope for calculation. subscopes are aggregated using the 'aggregation' function
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
	Metrics        []LikwidCollectorMetricConfig   `json:"globalmetrics"`
	ExcludeMetrics []string                        `json:"exclude_metrics"`
	ForceOverwrite bool                            `json:"force_overwrite"`
}

type LikwidCollector struct {
	metricCollector
	cpulist   []C.int
	sock2tid  map[int]int
	metrics   map[C.int]map[string]int
	groups    []C.int
	config    LikwidCollectorConfig
	results   map[int]map[int]map[string]interface{}
	mresults  map[int]map[int]map[string]float64
	gmresults map[int]map[string]float64
	basefreq  float64
	running   bool
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
		return "hwthread"
	} else if strings.Contains(counter, "BOX") || strings.Contains(counter, "DEV") {
		return "socket"
	} else if strings.HasPrefix(counter, "PWR") {
		if event == "RAPL_CORE_ENERGY" {
			return "hwthread"
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
		freq = float64(info.baseFrequency)
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

func getSocketCpus() map[C.int]int {
	slist := SocketList()
	var cpu C.int
	outmap := make(map[C.int]int)
	for _, s := range slist {
		t := C.CString(fmt.Sprintf("S%d", s))
		clen := C.cpustr_to_cpulist(t, &cpu, 1)
		if int(clen) == 1 {
			outmap[cpu] = s
		}
	}
	return outmap
}

func (m *LikwidCollector) CatchGvalPanic() {
	if rerr := recover(); rerr != nil {
		cclog.ComponentError(m.name, "Gval failed to calculate a metric", rerr)
		m.init = false
	}
}

func (m *LikwidCollector) initGranularity() {
	for _, evset := range m.config.Eventsets {
		evset.granulatity = make(map[string]MetricScope)
		for counter, event := range evset.Events {
			gran := getGranularity(counter, event)
			if gran.Granularity() >= 0 {
				evset.granulatity[counter] = gran
			}
		}
		for i, metric := range evset.Metrics {
			s := regexp.MustCompile("[+-/*()]").Split(metric.Calc, -1)
			gran := MetricScope("hwthread")
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
		s := regexp.MustCompile("[+-/*()]").Split(metric.Calc, -1)
		gran := MetricScope("hwthread")
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

func (m *LikwidCollector) Init(config json.RawMessage) error {
	var ret C.int
	m.name = "LikwidCollector"
	if len(config) > 0 {
		err := json.Unmarshal(config, &m.config)
		if err != nil {
			return err
		}
	}
	m.initGranularity()
	if m.config.ForceOverwrite {
		os.Setenv("LIKWID_FORCE", "1")
	}
	m.setup()
	// in some cases, gval causes a panic. We catch it with the handler and deactivate
	// the collector (m.init = false).
	defer m.CatchGvalPanic()

	m.meta = map[string]string{"source": m.name, "group": "PerfCounter"}
	cpulist := topo.CpuList()
	m.cpulist = make([]C.int, len(cpulist))

	cclog.ComponentDebug(m.name, "Create maps for socket, numa, core and die metrics")
	m.sock2tid = make(map[int]int)
	// m.numa2tid = make(map[int]int)
	// m.core2tid = make(map[int]int)
	// m.die2tid = make(map[int]int)
	for i, c := range cpulist {
		m.cpulist[i] = C.int(c)
		m.sock2tid[topo.GetCpuSocket(c)] = i
		// m.numa2tid[topo.GetCpuNumaDomain(c)] = i
		// m.core2tid[topo.GetCpuCore(c)] = i
		// m.die2tid[topo.GetCpuDie(c)] = i
	}
	m.results = make(map[int]map[int]map[string]interface{})
	m.mresults = make(map[int]map[int]map[string]float64)
	m.gmresults = make(map[int]map[string]float64)
	ret = C.topology_init()
	if ret != 0 {
		err := errors.New("failed to initialize LIKWID topology")
		cclog.ComponentError(m.name, err.Error())
		return err
	}
	ret = C.perfmon_init(C.int(len(m.cpulist)), &m.cpulist[0])
	if ret != 0 {
		C.topology_finalize()
		err := errors.New("failed to initialize LIKWID topology")
		cclog.ComponentError(m.name, err.Error())
		return err
	}

	globalParams := make(map[string]interface{})
	globalParams["time"] = float64(1.0)
	globalParams["inverseClock"] = float64(1.0)

	for i, evset := range m.config.Eventsets {
		estr := eventsToEventStr(evset.Events)
		params := make(map[string]interface{})
		params["time"] = float64(1.0)
		params["inverseClock"] = float64(1.0)
		for counter, _ := range evset.Events {
			params[counter] = float64(1.0)
		}
		for _, metric := range evset.Metrics {
			_, err := gval.Evaluate(metric.Calc, params, gval.Full())
			if err != nil {
				cclog.ComponentError(m.name, "Calculation for metric", metric.Name, "failed:", err.Error())
				continue
			}
			if _, ok := globalParams[metric.Name]; !ok {
				globalParams[metric.Name] = float64(1.0)
			}
		}
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
			m.gmresults[tid] = make(map[string]float64)
		}
	}
	for _, metric := range m.config.Metrics {
		_, err := gval.Evaluate(metric.Calc, globalParams, gval.Full())
		if err != nil {
			cclog.ComponentError(m.name, "Calculation for metric", metric.Name, "failed:", err.Error())
			continue
		}
	}

	if len(m.groups) == 0 {
		C.perfmon_finalize()
		C.topology_finalize()
		err := errors.New("no LIKWID performance group initialized")
		cclog.ComponentError(m.name, err.Error())
		return err
	}
	m.basefreq = getBaseFreq()
	m.init = true
	return nil
}

func (m *LikwidCollector) takeMeasurement(group int, interval time.Duration) error {
	var ret C.int
	gid := m.groups[group]
	ret = C.perfmon_setupCounters(gid)
	if ret != 0 {
		gctr := C.GoString(C.perfmon_getGroupName(gid))
		err := fmt.Errorf("failed to setup performance group %s", gctr)
		cclog.ComponentError(m.name, err.Error())
		return err
	}
	ret = C.perfmon_startCounters()
	if ret != 0 {
		gctr := C.GoString(C.perfmon_getGroupName(gid))
		err := fmt.Errorf("failed to start performance group %s", gctr)
		cclog.ComponentError(m.name, err.Error())
		return err
	}
	m.running = true
	time.Sleep(interval)
	m.running = false
	ret = C.perfmon_stopCounters()
	if ret != 0 {
		gctr := C.GoString(C.perfmon_getGroupName(gid))
		err := fmt.Errorf("failed to stop performance group %s", gctr)
		cclog.ComponentError(m.name, err.Error())
		return err
	}
	return nil
}

func (m *LikwidCollector) calcEventsetMetrics(group int, interval time.Duration) error {
	var eidx C.int
	evset := m.config.Eventsets[group]
	gid := m.groups[group]
	for tid := range m.cpulist {
		for eidx = 0; int(eidx) < len(evset.Events); eidx++ {
			ctr := C.perfmon_getCounterName(gid, eidx)
			gctr := C.GoString(ctr)
			res := C.perfmon_getLastResult(gid, eidx, C.int(tid))
			m.results[group][tid][gctr] = float64(res)
			if m.results[group][tid][gctr] == 0 {
				m.results[group][tid][gctr] = 1.0
			}
		}
		m.results[group][tid]["time"] = interval.Seconds()
		m.results[group][tid]["inverseClock"] = float64(1.0 / m.basefreq)
		for _, metric := range evset.Metrics {
			value, err := gval.Evaluate(metric.Calc, m.results[group][tid], gval.Full())
			if err != nil {
				cclog.ComponentError(m.name, "Calculation for metric", metric.Name, "failed:", err.Error())
				continue
			}
			m.mresults[group][tid][metric.Name] = value.(float64)
		}
	}
	return nil
}

func (m *LikwidCollector) calcGlobalMetrics(interval time.Duration) error {
	for _, metric := range m.config.Metrics {
		for tid := range m.cpulist {
			params := make(map[string]interface{})
			for j := range m.groups {
				for mname, mres := range m.mresults[j][tid] {
					params[mname] = mres
				}
			}
			value, err := gval.Evaluate(metric.Calc, params, gval.Full())
			if err != nil {
				cclog.ComponentError(m.name, "Calculation for metric", metric.Name, "failed:", err.Error())
				continue
			}
			m.gmresults[tid][metric.Name] = value.(float64)
		}
	}
	return nil
}

// func (m *LikwidCollector) calcResultMetrics(interval time.Duration) ([]lp.CCMetric, error) {
// 	var err error = nil
// 	metrics := make([]lp.CCMetric, 0)
// 	for i := range m.groups {
// 		evset := m.config.Eventsets[i]
// 		for _, metric := range evset.Metrics {
// 			log.Print(metric.Name, " ", metric.Scope, " ", metric.granulatity)
// 			if metric.Scope.Granularity() > metric.granulatity.Granularity() {
// 				log.Print("Different granularity wanted for ", metric.Name, ": ", metric.Scope, " vs ", metric.granulatity)
// 				var idlist []int
// 				idfunc := func(cpuid int) int { return cpuid }
// 				switch metric.Scope {
// 				case "socket":
// 					idlist = topo.SocketList()
// 					idfunc = topo.GetCpuSocket
// 				case "numa":
// 					idlist = topo.NumaNodeList()
// 					idfunc = topo.GetCpuNumaDomain
// 				case "core":
// 					idlist = topo.CoreList()
// 					idfunc = topo.GetCpuCore
// 				case "die":
// 					idlist = topo.DieList()
// 					idfunc = topo.GetCpuDie
// 				case "node":
// 					idlist = topo.CpuList()
// 				}
// 				for i := 0; i < num_results; i++ {

// 				}
// 			}
// 		}
// 	}
// 	for _, metric := range m.config.Metrics {
// 		log.Print(metric.Name, " ", metric.Scope, " ", metric.granulatity)
// 		if metric.Scope.Granularity() > metric.granulatity.Granularity() {
// 			log.Print("Different granularity wanted for ", metric.Name, ": ", metric.Scope, " vs ", metric.granulatity)
// 		}
// 	}
// 	return metrics, err
// }

func (m *LikwidCollector) Read(interval time.Duration, output chan lp.CCMetric) {
	if !m.init {
		return
	}
	defer m.CatchGvalPanic()

	for i, _ := range m.groups {
		// measure event set 'i' for 'interval' seconds
		err := m.takeMeasurement(i, interval)
		if err != nil {
			cclog.ComponentError(m.name, err.Error())
			continue
		}
		m.calcEventsetMetrics(i, interval)
	}

	m.calcGlobalMetrics(interval)

	//metrics, err = m.calcResultMetrics(interval)

	for i := range m.groups {
		evset := m.config.Eventsets[i]
		for _, metric := range evset.Metrics {

			_, skip := stringArrayContains(m.config.ExcludeMetrics, metric.Name)
			if metric.Publish && !skip {
				if metric.Scope == "socket" {
					for sid, tid := range m.sock2tid {
						y, err := lp.New(metric.Name,
							map[string]string{"type": "socket",
								"type-id": fmt.Sprintf("%d", int(sid))},
							m.meta,
							map[string]interface{}{"value": m.mresults[i][tid][metric.Name]},
							time.Now())
						if err == nil {
							output <- y
						}
					}
				} else if metric.Scope == "hwthread" {
					for tid, cpu := range m.cpulist {
						y, err := lp.New(metric.Name,
							map[string]string{"type": "cpu",
								"type-id": fmt.Sprintf("%d", int(cpu))},
							m.meta,
							map[string]interface{}{"value": m.mresults[i][tid][metric.Name]},
							time.Now())
						if err == nil {
							output <- y
						}
					}
				}
			}
		}
	}
	for _, metric := range m.config.Metrics {
		_, skip := stringArrayContains(m.config.ExcludeMetrics, metric.Name)
		if metric.Publish && !skip {
			if metric.Scope == "socket" {
				for sid, tid := range m.sock2tid {
					y, err := lp.New(metric.Name,
						map[string]string{"type": "socket",
							"type-id": fmt.Sprintf("%d", int(sid))},
						m.meta,
						map[string]interface{}{"value": m.gmresults[tid][metric.Name]},
						time.Now())
					if err == nil {
						output <- y
					}
				}
			} else if metric.Scope == "hwthread" {
				for tid, cpu := range m.cpulist {
					y, err := lp.New(metric.Name,
						map[string]string{"type": "cpu",
							"type-id": fmt.Sprintf("%d", int(cpu))},
						m.meta,
						map[string]interface{}{"value": m.gmresults[tid][metric.Name]},
						time.Now())
					if err == nil {
						output <- y
					}
				}
			}
		}
	}
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
