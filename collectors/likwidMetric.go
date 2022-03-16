package collectors

/*
#cgo CFLAGS: -I./likwid
#cgo LDFLAGS: -Wl,--unresolved-symbols=ignore-in-object-files
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

const (
	LIKWID_LIB_NAME       = "liblikwid.so"
	LIKWID_LIB_DL_FLAGS   = dl.RTLD_LAZY | dl.RTLD_GLOBAL
	LIKWID_DEF_ACCESSMODE = "direct"
)

type LikwidCollectorMetricConfig struct {
	Name    string `json:"name"` // Name of the metric
	Calc    string `json:"calc"` // Calculation for the metric using
	Type    string `json:"type"` // Metric type (aka node, socket, cpu, ...)
	Publish bool   `json:"publish"`
	Unit    string `json:"unit"` // Unit of metric if any
}

type LikwidCollectorEventsetConfig struct {
	Events  map[string]string             `json:"events"`
	Metrics []LikwidCollectorMetricConfig `json:"metrics"`
}

type LikwidCollectorConfig struct {
	Eventsets      []LikwidCollectorEventsetConfig `json:"eventsets"`
	Metrics        []LikwidCollectorMetricConfig   `json:"globalmetrics,omitempty"`
	ForceOverwrite bool                            `json:"force_overwrite,omitempty"`
	InvalidToZero  bool                            `json:"invalid_to_zero,omitempty"`
	AccessMode     string                          `json:"access_mode,omitempty"`
	DaemonPath     string                          `json:"accessdaemon_path,omitempty"`
	LibraryPath    string                          `json:"liblikwid_path,omitempty"`
}

type LikwidCollector struct {
	metricCollector
	cpulist   []C.int
	cpu2tid   map[int]int
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
	scope     string
	group_idx int
}

func eventsToEventStr(events map[string]string) string {
	elist := make([]string, 0)
	for k, v := range events {
		elist = append(elist, fmt.Sprintf("%s:%s", v, k))
	}
	return strings.Join(elist, ",")
}

func getBaseFreq() float64 {
	var freq float64 = math.NaN()
	C.power_init(0)
	info := C.get_powerInfo()
	if float64(info.baseFrequency) != 0 {
		freq = float64(info.baseFrequency) * 1e6
	} else {
		buffer, err := ioutil.ReadFile("/sys/devices/system/cpu/cpu0/cpufreq/bios_limit")
		if err == nil {
			data := strings.Replace(string(buffer), "\n", "", -1)
			x, err := strconv.ParseInt(data, 0, 64)
			if err == nil {
				freq = float64(x) * 1e6
			}
		}
	}
	return freq
}

func (m *LikwidCollector) Init(config json.RawMessage) error {
	var ret C.int
	m.name = "LikwidCollector"
	m.config.AccessMode = LIKWID_DEF_ACCESSMODE
	m.config.LibraryPath = LIKWID_LIB_NAME
	if len(config) > 0 {
		err := json.Unmarshal(config, &m.config)
		if err != nil {
			return err
		}
	}
	lib := dl.New(m.config.LibraryPath, LIKWID_LIB_DL_FLAGS)
	if lib == nil {
		return fmt.Errorf("error instantiating DynamicLibrary for %s", m.config.LibraryPath)
	}
	err := lib.Open()
	if err != nil {
		return fmt.Errorf("error opening %s: %v", m.config.LibraryPath, err)
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
	m.sock2tid = make(map[int]int)
	tmp := make([]C.int, 1)
	for _, sid := range topo.SocketList() {
		cstr := C.CString(fmt.Sprintf("S%d:0", sid))
		ret = C.cpustr_to_cpulist(cstr, &tmp[0], 1)
		if ret > 0 {
			m.sock2tid[sid] = m.cpu2tid[int(tmp[0])]
		}
		C.free(unsafe.Pointer(cstr))
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

	switch m.config.AccessMode {
	case "direct":
		C.HPMmode(0)
	case "accessdaemon":
		if len(m.config.DaemonPath) > 0 {
			p := os.Getenv("PATH")
			os.Setenv("PATH", m.config.DaemonPath+":"+p)
		}
		C.HPMmode(1)
	}

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
		var gid C.int
		var cstr *C.char
		if len(evset.Events) > 0 {
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
			cstr = C.CString(estr)
			gid = C.perfmon_addEventSet(cstr)
		} else {
			cclog.ComponentError(m.name, "Invalid Likwid eventset config, no events given")
			continue
		}
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
		gctr := C.GoString(ctr)

		for _, tid := range m.cpu2tid {
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
		scopemap := m.cpu2tid
		if metric.Type == "socket" {
			scopemap = m.sock2tid
		}
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
						fields := map[string]interface{}{"value": value}
						y, err := lp.New(metric.Name, map[string]string{"type": metric.Type}, m.meta, fields, time.Now())
						if err == nil {
							if metric.Type != "node" {
								y.AddTag("type-id", fmt.Sprintf("%d", domain))
							}
							if len(metric.Unit) > 0 {
								y.AddMeta("unit", metric.Unit)
							}
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
		scopemap := m.cpu2tid
		if metric.Type == "socket" {
			scopemap = m.sock2tid
		}
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
						tags := map[string]string{"type": metric.Type}
						fields := map[string]interface{}{"value": value}
						y, err := lp.New(metric.Name, tags, m.meta, fields, time.Now())
						if err == nil {
							if metric.Type != "node" {
								y.AddTag("type-id", fmt.Sprintf("%d", domain))
							}
							if len(metric.Unit) > 0 {
								y.AddMeta("unit", metric.Unit)
							}
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
