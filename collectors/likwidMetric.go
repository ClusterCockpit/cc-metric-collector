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
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
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

type LikwidEventsetConfig struct {
	internal int
	gid      C.int
	eorder   []*C.char
	estr     *C.char
	go_estr  string
	results  map[int]map[string]interface{}
	metrics  map[int]map[string]float64
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
	cpulist      []C.int
	cpu2tid      map[int]int
	sock2tid     map[int]int
	metrics      map[C.int]map[string]int
	groups       []C.int
	config       LikwidCollectorConfig
	gmresults    map[int]map[string]float64
	basefreq     float64
	running      bool
	initialized  bool
	likwidGroups map[C.int]LikwidEventsetConfig
	lock         sync.Mutex
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

func genLikwidEventSet(input LikwidCollectorEventsetConfig) LikwidEventsetConfig {
	tmplist := make([]string, 0)
	clist := make([]string, 0)
	for k := range input.Events {
		clist = append(clist, k)
	}
	sort.Strings(clist)
	elist := make([]*C.char, 0)
	for _, k := range clist {
		v := input.Events[k]
		tmplist = append(tmplist, fmt.Sprintf("%s:%s", v, k))
		c_counter := C.CString(k)
		elist = append(elist, c_counter)
	}
	estr := strings.Join(tmplist, ",")
	res := make(map[int]map[string]interface{})
	met := make(map[int]map[string]float64)
	for _, i := range topo.CpuList() {
		res[i] = make(map[string]interface{})
		for k := range input.Events {
			res[i][k] = 0.0
		}
		met[i] = make(map[string]float64)
		for _, v := range input.Metrics {
			res[i][v.Name] = 0.0
		}
	}
	return LikwidEventsetConfig{
		gid:     -1,
		eorder:  elist,
		estr:    C.CString(estr),
		go_estr: estr,
		results: res,
		metrics: met,
	}
}

func testLikwidMetricFormula(formula string, params []string) bool {
	myparams := make(map[string]interface{})
	for _, p := range params {
		myparams[p] = float64(1.0)
	}
	_, err := agg.EvalFloat64Condition(formula, myparams)
	return err == nil
}

func getBaseFreq() float64 {
	files := []string{
		"/sys/devices/system/cpu/cpu0/cpufreq/bios_limit",
		"/sys/devices/system/cpu/cpu0/cpufreq/base_frequency",
	}
	var freq float64 = math.NaN()
	for _, f := range files {
		buffer, err := ioutil.ReadFile(f)
		if err == nil {
			data := strings.Replace(string(buffer), "\n", "", -1)
			x, err := strconv.ParseInt(data, 0, 64)
			if err == nil {
				freq = float64(x)
				break
			}
		}
	}

	if math.IsNaN(freq) {
		C.power_init(0)
		info := C.get_powerInfo()
		if float64(info.baseFrequency) != 0 {
			freq = float64(info.baseFrequency)
		}
		C.power_finalize()
	}
	return freq * 1e3
}

func (m *LikwidCollector) Init(config json.RawMessage) error {
	m.name = "LikwidCollector"
	m.parallel = false
	m.initialized = false
	m.running = false
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

	m.meta = map[string]string{"group": "PerfCounter"}
	cclog.ComponentDebug(m.name, "Get cpulist and init maps and lists")
	cpulist := topo.HwthreadList()
	m.cpulist = make([]C.int, len(cpulist))
	m.cpu2tid = make(map[int]int)
	for i, c := range cpulist {
		m.cpulist[i] = C.int(c)
		m.cpu2tid[c] = i
	}

	m.likwidGroups = make(map[C.int]LikwidEventsetConfig)

	// m.results = make(map[int]map[int]map[string]interface{})
	// m.mresults = make(map[int]map[int]map[string]float64)
	m.gmresults = make(map[int]map[string]float64)
	for _, tid := range m.cpu2tid {
		m.gmresults[tid] = make(map[string]float64)
	}

	// This is for the global metrics computation test
	totalMetrics := 0
	// Generate parameter list for the metric computing test
	params := make([]string, 0)
	params = append(params, "time", "inverseClock")
	// Generate parameter list for the global metric computing test
	globalParams := make([]string, 0)
	globalParams = append(globalParams, "time", "inverseClock")
	// We test the eventset metrics whether they can be computed at all
	for _, evset := range m.config.Eventsets {
		if len(evset.Events) > 0 {
			params = params[:2]
			for counter := range evset.Events {
				params = append(params, counter)
			}
			for _, metric := range evset.Metrics {
				// Try to evaluate the metric
				if testLikwidMetricFormula(metric.Calc, params) {
					// Add the computable metric to the parameter list for the global metrics
					globalParams = append(globalParams, metric.Name)
					totalMetrics++
				} else {
					metric.Calc = ""
				}
			}
		} else {
			cclog.ComponentError(m.name, "Invalid Likwid eventset config, no events given")
			continue
		}
	}
	for _, metric := range m.config.Metrics {
		// Try to evaluate the global metric
		if !testLikwidMetricFormula(metric.Calc, globalParams) {
			cclog.ComponentError(m.name, "Calculation for metric", metric.Name, "failed")
			metric.Calc = ""
		} else {
			totalMetrics++
		}
	}

	// If no event set could be added, shut down LikwidCollector
	if totalMetrics == 0 {
		err := errors.New("no LIKWID eventset or metric usable")
		cclog.ComponentError(m.name, err.Error())
		return err
	}
	m.init = true
	return nil
}

// take a measurement for 'interval' seconds of event set index 'group'
func (m *LikwidCollector) takeMeasurement(evset LikwidEventsetConfig, interval time.Duration) (bool, error) {
	var ret C.int
	m.lock.Lock()
	if m.initialized {
		ret = C.perfmon_setupCounters(evset.gid)
		if ret != 0 {
			var err error = nil
			var skip bool = false
			if ret == -37 {
				skip = true
			} else {
				err = fmt.Errorf("failed to setup performance group %d", evset.gid)
			}
			m.lock.Unlock()
			return skip, err
		}
		ret = C.perfmon_startCounters()
		if ret != 0 {
			var err error = nil
			var skip bool = false
			if ret == -37 {
				skip = true
			} else {
				err = fmt.Errorf("failed to setup performance group %d", evset.gid)
			}
			m.lock.Unlock()
			return skip, err
		}
		m.running = true
		time.Sleep(interval)
		m.running = false
		ret = C.perfmon_stopCounters()
		if ret != 0 {
			var err error = nil
			var skip bool = false
			if ret == -37 {
				skip = true
			} else {
				err = fmt.Errorf("failed to setup performance group %d", evset.gid)
			}
			m.lock.Unlock()
			return skip, err
		}
	}
	m.lock.Unlock()
	return false, nil
}

// Get all measurement results for an event set, derive the metric values out of the measurement results and send it
func (m *LikwidCollector) calcEventsetMetrics(evset LikwidEventsetConfig, interval time.Duration, output chan lp.CCMetric) error {
	invClock := float64(1.0 / m.basefreq)

	// Go over events and get the results
	for eidx, counter := range evset.eorder {
		gctr := C.GoString(counter)
		for _, tid := range m.cpu2tid {
			res := C.perfmon_getLastResult(evset.gid, C.int(eidx), C.int(tid))
			fres := float64(res)
			if m.config.InvalidToZero && (math.IsNaN(fres) || math.IsInf(fres, 0)) {
				fres = 0.0
			}
			evset.results[tid][gctr] = fres
			evset.results[tid]["time"] = interval.Seconds()
			evset.results[tid]["inverseClock"] = invClock
		}
	}

	// Go over the event set metrics, derive the value out of the event:counter values and send it
	for _, metric := range m.config.Eventsets[evset.internal].Metrics {
		// The metric scope is determined in the Init() function
		// Get the map scope-id -> tids
		scopemap := m.cpu2tid
		if metric.Type == "socket" {
			scopemap = m.sock2tid
		}
		for domain, tid := range scopemap {
			if tid >= 0 && len(metric.Calc) > 0 {
				value, err := agg.EvalFloat64Condition(metric.Calc, evset.results[tid])
				if err != nil {
					cclog.ComponentError(m.name, "Calculation for metric", metric.Name, "failed:", err.Error())
					value = 0.0
				}
				if m.config.InvalidToZero && (math.IsNaN(value) || math.IsInf(value, 0)) {
					value = 0.0
				}
				evset.metrics[tid][metric.Name] = value
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
				for _, evset := range m.likwidGroups {
					for mname, mres := range evset.metrics[tid] {
						params[mname] = mres
					}
				}
				// Evaluate the metric
				value, err := agg.EvalFloat64Condition(metric.Calc, params)
				if err != nil {
					cclog.ComponentError(m.name, "Calculation for metric", metric.Name, "failed:", err.Error())
					value = 0.0
				}
				if m.config.InvalidToZero && (math.IsNaN(value) || math.IsInf(value, 0)) {
					value = 0.0
				}
				m.gmresults[tid][metric.Name] = value
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

func (m *LikwidCollector) LateInit() error {
	var ret C.int
	if m.initialized {
		return nil
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
	cclog.ComponentDebug(m.name, "initialize LIKWID topology")
	ret = C.topology_init()
	if ret != 0 {
		err := errors.New("failed to initialize LIKWID topology")
		cclog.ComponentError(m.name, err.Error())
		return err
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

	m.basefreq = getBaseFreq()
	cclog.ComponentDebug(m.name, "BaseFreq", m.basefreq)

	cclog.ComponentDebug(m.name, "initialize LIKWID perfmon module")
	ret = C.perfmon_init(C.int(len(m.cpulist)), &m.cpulist[0])
	if ret != 0 {
		var err error = nil
		C.topology_finalize()
		if ret != -22 {
			err = errors.New("failed to initialize LIKWID perfmon")
			cclog.ComponentError(m.name, err.Error())
		} else {
			err = errors.New("access to LIKWID perfmon locked")
		}
		return err
	}

	// While adding the events, we test the metrics whether they can be computed at all
	for i, evset := range m.config.Eventsets {
		var gid C.int
		if len(evset.Events) > 0 {
			skip := false
			likwidGroup := genLikwidEventSet(evset)
			for _, g := range m.likwidGroups {
				if likwidGroup.go_estr == g.go_estr {
					skip = true
					break
				}
			}
			if skip {
				continue
			}
			// Now we add the list of events to likwid
			gid = C.perfmon_addEventSet(likwidGroup.estr)
			if gid >= 0 {
				likwidGroup.gid = gid
				likwidGroup.internal = i
				m.likwidGroups[gid] = likwidGroup
			}
		} else {
			cclog.ComponentError(m.name, "Invalid Likwid eventset config, no events given")
			continue
		}

	}

	// If no event set could be added, shut down LikwidCollector
	if len(m.likwidGroups) == 0 {
		C.perfmon_finalize()
		C.topology_finalize()
		err := errors.New("no LIKWID performance group initialized")
		cclog.ComponentError(m.name, err.Error())
		return err
	}
	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, syscall.SIGCHLD)
	signal.Notify(sigchan, os.Interrupt)
	go func() {
		<-sigchan

		signal.Stop(sigchan)
		m.initialized = false
	}()
	m.initialized = true
	return nil
}

// main read function taking multiple measurement rounds, each 'interval' seconds long
func (m *LikwidCollector) Read(interval time.Duration, output chan lp.CCMetric) {
	var skip bool = false
	var err error
	if !m.init {
		return
	}

	if !m.initialized {
		m.lock.Lock()
		err = m.LateInit()
		if err != nil {
			m.lock.Unlock()
			return
		}
		m.initialized = true
		m.lock.Unlock()
	}

	if m.initialized && !skip {
		for _, evset := range m.likwidGroups {
			if !skip {
				// measure event set 'i' for 'interval' seconds
				skip, err = m.takeMeasurement(evset, interval)
				if err != nil {
					cclog.ComponentError(m.name, err.Error())
					return
				}
			}

			if !skip {
				// read measurements and derive event set metrics
				m.calcEventsetMetrics(evset, interval, output)
			}
		}
		if !skip {
			// use the event set metrics to derive the global metrics
			m.calcGlobalMetrics(interval, output)
		}
	}
}

func (m *LikwidCollector) Close() {
	if m.init {
		m.init = false
		cclog.ComponentDebug(m.name, "Closing ...")
		m.lock.Lock()
		if m.initialized {
			cclog.ComponentDebug(m.name, "Finalize LIKWID perfmon module")
			C.perfmon_finalize()
			m.initialized = false
		}
		m.lock.Unlock()
		cclog.ComponentDebug(m.name, "Finalize LIKWID topology module")
		C.topology_finalize()

		cclog.ComponentDebug(m.name, "Closing done")
	}
}
