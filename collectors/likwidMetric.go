// Copyright (C) NHR@FAU, University Erlangen-Nuremberg.
// All rights reserved. This file is part of cc-lib.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.
// additional authors:
// Holger Obermaier (NHR@KIT)

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
	"maps"
	"math"
	"os"
	"os/signal"
	"os/user"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	cclog "github.com/ClusterCockpit/cc-lib/v2/ccLogger"
	lp "github.com/ClusterCockpit/cc-lib/v2/ccMessage"
	agg "github.com/ClusterCockpit/cc-metric-collector/internal/metricAggregator"
	topo "github.com/ClusterCockpit/cc-metric-collector/pkg/ccTopology"
	"github.com/NVIDIA/go-nvml/pkg/dl"
	"github.com/fsnotify/fsnotify"
	"golang.design/x/thread"
)

const (
	LIKWID_LIB_NAME       = "liblikwid.so"
	LIKWID_LIB_DL_FLAGS   = dl.RTLD_LAZY | dl.RTLD_GLOBAL
	LIKWID_DEF_ACCESSMODE = "direct"
	LIKWID_DEF_LOCKFILE   = "/var/run/likwid.lock"
)

type LikwidCollectorMetricConfig struct {
	Name               string `json:"name"` // Name of the metric
	Calc               string `json:"calc"` // Calculation for the metric using
	Type               string `json:"type"` // Metric type (aka node, socket, hwthread, ...)
	Publish            bool   `json:"publish"`
	SendCoreTotalVal   bool   `json:"send_core_total_values,omitempty"`
	SendSocketTotalVal bool   `json:"send_socket_total_values,omitempty"`
	SendNodeTotalVal   bool   `json:"send_node_total_values,omitempty"`
	Unit               string `json:"unit"` // Unit of metric if any
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
	results  map[int]map[string]float64
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
	LockfilePath   string                          `json:"lockfile_path,omitempty"`
}

type LikwidCollector struct {
	metricCollector
	cpulist       []C.int
	cpu2tid       map[int]int
	sock2tid      map[int]int
	tid2core      map[int]int
	tid2socket    map[int]int
	metrics       map[C.int]map[string]int
	groups        []C.int
	config        LikwidCollectorConfig
	basefreq      float64
	running       bool
	initialized   bool
	needs_reinit  bool
	myuid         int
	lock_err_once bool
	likwidGroups  map[C.int]LikwidEventsetConfig
	lock          sync.Mutex
	measureThread thread.Thread
}

type LikwidMetric struct {
	name      string
	search    string
	scope     string
	group_idx int
}

func checkMetricType(t string) bool {
	valid := map[string]bool{
		"node":         true,
		"socket":       true,
		"hwthread":     true,
		"core":         true,
		"memoryDomain": true,
	}
	_, ok := valid[t]
	return ok
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
	res := make(map[int]map[string]float64)
	met := make(map[int]map[string]float64)
	for _, i := range topo.CpuList() {
		res[i] = make(map[string]float64)
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
	myparams := make(map[string]float64)
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
		buffer, err := os.ReadFile(f)
		if err == nil {
			data := strings.ReplaceAll(string(buffer), "\n", "")
			x, err := strconv.ParseInt(data, 0, 64)
			if err == nil {
				freq = float64(x)
				break
			}
		}
	}

	if math.IsNaN(freq) {
		C.timer_init()
		freq = float64(C.timer_getCycleClock()) / 1e3
	}
	return freq * 1e3
}

func (m *LikwidCollector) Init(config json.RawMessage) error {
	m.name = "LikwidCollector"
	m.parallel = false
	m.initialized = false
	m.needs_reinit = true
	m.running = false
	m.myuid = os.Getuid()
	m.config.AccessMode = LIKWID_DEF_ACCESSMODE
	m.config.LibraryPath = LIKWID_LIB_NAME
	m.config.LockfilePath = LIKWID_DEF_LOCKFILE
	if len(config) > 0 {
		err := json.Unmarshal(config, &m.config)
		if err != nil {
			return fmt.Errorf("%s Init(): failed to unmarshal JSON config: %w", m.name, err)
		}
	}
	lib := dl.New(m.config.LibraryPath, LIKWID_LIB_DL_FLAGS)
	if lib == nil {
		return fmt.Errorf("error instantiating DynamicLibrary for %s", m.config.LibraryPath)
	}
	err := lib.Open()
	if err != nil {
		return fmt.Errorf("error opening %s: %w", m.config.LibraryPath, err)
	}

	if m.config.ForceOverwrite {
		cclog.ComponentDebug(m.name, "Set LIKWID_FORCE=1")
		if err := os.Setenv("LIKWID_FORCE", "1"); err != nil {
			return fmt.Errorf("error setting environment variable LIKWID_FORCE=1: %w", err)
		}
	}
	if err := m.setup(); err != nil {
		return fmt.Errorf("%s Init(): setup() call failed: %w", m.name, err)
	}

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
				cclog.ComponentDebug(m.name, "Checking", metric.Name)
				if !checkMetricType(metric.Type) {
					cclog.ComponentError(m.name, "Metric", metric.Name, "uses invalid type", metric.Type)
					metric.Calc = ""
				} else if !testLikwidMetricFormula(metric.Calc, params) {
					cclog.ComponentError(m.name, "Metric", metric.Name, "cannot be calculated with given counters")
					metric.Calc = ""
				} else {
					globalParams = append(globalParams, metric.Name)
					totalMetrics++
				}
			}
		} else {
			cclog.ComponentError(m.name, "Invalid Likwid eventset config, no events given")
			continue
		}
	}
	for _, metric := range m.config.Metrics {
		// Try to evaluate the global metric
		if !checkMetricType(metric.Type) {
			cclog.ComponentError(m.name, "Metric", metric.Name, "uses invalid type", metric.Type)
			metric.Calc = ""
		} else if !testLikwidMetricFormula(metric.Calc, globalParams) {
			cclog.ComponentError(m.name, "Metric", metric.Name, "cannot be calculated with given counters")
			metric.Calc = ""
		} else if !checkMetricType(metric.Type) {
			cclog.ComponentError(m.name, "Metric", metric.Name, "has invalid type")
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

	ret := C.topology_init()
	if ret != 0 {
		err := errors.New("failed to initialize topology module")
		cclog.ComponentError(m.name, err.Error())
		return err
	}
	m.measureThread = thread.New()
	switch m.config.AccessMode {
	case "direct":
		C.HPMmode(0)
	case "accessdaemon":
		if len(m.config.DaemonPath) > 0 {
			p := os.Getenv("PATH")
			if len(p) > 0 {
				p = m.config.DaemonPath + ":" + p
			} else {
				p = m.config.DaemonPath
			}
			if err := os.Setenv("PATH", p); err != nil {
				return fmt.Errorf("error setting environment variable PATH=%s: %w", p, err)
			}
		}
		C.HPMmode(1)
		retCode := C.HPMinit()
		if retCode != 0 {
			err := fmt.Errorf("C.HPMinit() failed with return code %v", retCode)
			cclog.ComponentError(m.name, err.Error())
		}
		for _, c := range m.cpulist {
			m.measureThread.Call(
				func() {
					retCode := C.HPMaddThread(c)
					if retCode != 0 {
						err := fmt.Errorf("C.HPMaddThread(%v) failed with return code %v", c, retCode)
						cclog.ComponentError(m.name, err.Error())
					}
				})
		}
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

	cpuData := topo.CpuData()
	m.tid2core = make(map[int]int, len(cpuData))
	m.tid2socket = make(map[int]int, len(cpuData))
	for i := range cpuData {
		c := &cpuData[i]
		// Hardware thread ID to core ID mapping
		if len(c.CoreCPUsList) > 0 {
			m.tid2core[c.CpuID] = c.CoreCPUsList[0]
		} else {
			m.tid2core[c.CpuID] = c.CpuID
		}
		// Hardware thead ID to socket ID mapping
		m.tid2socket[c.CpuID] = c.Socket
	}

	m.basefreq = getBaseFreq()
	m.init = true
	return nil
}

// take a measurement for 'interval' seconds of event set index 'group'
func (m *LikwidCollector) takeMeasurement(evidx int, evset LikwidEventsetConfig, interval time.Duration) (bool, error) {
	var ret C.int
	var gid C.int = -1
	sigchan := make(chan os.Signal, 1)

	// Watch changes for the lock file ()
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		cclog.ComponentError(
			m.name,
			fmt.Sprintf("takeMeasurement(): Failed to create a new fsnotify.Watcher: %v", err))
		return true, err
	}
	defer func() {
		if err := watcher.Close(); err != nil {
			cclog.ComponentError(
				m.name,
				fmt.Sprintf("takeMeasurement(): Failed to close fsnotify.Watcher: %v", err))
		}
	}()
	if len(m.config.LockfilePath) > 0 {
		// Check if the lock file exists
		info, err := os.Stat(m.config.LockfilePath)
		if os.IsNotExist(err) {
			// Create the lock file if it does not exist
			file, createErr := os.Create(m.config.LockfilePath)
			if createErr != nil {
				return true, fmt.Errorf("failed to create lock file: %w", createErr)
			}
			if err := file.Close(); err != nil {
				return true, fmt.Errorf("failed to close lock file: %w", err)
			}
			info, err = os.Stat(m.config.LockfilePath) // Recheck the file after creation
		}
		if err != nil {
			return true, err
		}
		// Check file ownership
		uid := info.Sys().(*syscall.Stat_t).Uid
		if uid != uint32(m.myuid) {
			usr, err := user.LookupId(fmt.Sprint(uid))
			if err == nil {
				err = fmt.Errorf("access to performance counters locked by %s", usr.Username)
			} else {
				err = fmt.Errorf("access to performance counters locked by %d", uid)
			}
			// delete error if we already returned the error once.
			if !m.lock_err_once {
				m.lock_err_once = true
			} else {
				err = nil
			}
			return true, err
		}
		// reset lock_err_once
		m.lock_err_once = false

		// Add the lock file to the watcher
		err = watcher.Add(m.config.LockfilePath)
		if err != nil {
			cclog.ComponentError(m.name, err.Error())
		}
	}
	m.lock.Lock()
	defer m.lock.Unlock()

	// Initialize the performance monitoring feature by creating basic data structures
	select {
	case e := <-watcher.Events:
		ret = -1
		if e.Op != fsnotify.Chmod {
			ret = C.perfmon_init(C.int(len(m.cpulist)), &m.cpulist[0])
		}
	default:
		ret = C.perfmon_init(C.int(len(m.cpulist)), &m.cpulist[0])
	}
	if ret != 0 {
		return true, fmt.Errorf("failed to initialize library, error %d", ret)
	}
	signal.Notify(sigchan, os.Interrupt)
	signal.Notify(sigchan, syscall.SIGCHLD)

	// Add an event string to LIKWID
	select {
	case <-sigchan:
		gid = -1
	case e := <-watcher.Events:
		gid = -1
		if e.Op != fsnotify.Chmod {
			gid = C.perfmon_addEventSet(evset.estr)
		}
	default:
		gid = C.perfmon_addEventSet(evset.estr)
	}
	if gid < 0 {
		return true, fmt.Errorf("failed to add events %s, id %d, error %d", evset.go_estr, evidx, gid)
	}

	// Setup all performance monitoring counters of an eventSet
	select {
	case <-sigchan:
		ret = -1
	case e := <-watcher.Events:
		if e.Op != fsnotify.Chmod {
			ret = C.perfmon_setupCounters(gid)
		}
	default:
		ret = C.perfmon_setupCounters(gid)
	}
	if ret != 0 {
		return true, fmt.Errorf("failed to setup events '%s', error %d", evset.go_estr, ret)
	}

	// Start counters
	select {
	case <-sigchan:
		ret = -1
	case e := <-watcher.Events:
		if e.Op != fsnotify.Chmod {
			ret = C.perfmon_startCounters()
		}
	default:
		ret = C.perfmon_startCounters()
	}
	if ret != 0 {
		return true, fmt.Errorf("failed to start events '%s', error %d", evset.go_estr, ret)
	}
	select {
	case <-sigchan:
		ret = -1
	case e := <-watcher.Events:
		if e.Op != fsnotify.Chmod {
			ret = C.perfmon_readCounters()
		}
	default:
		ret = C.perfmon_readCounters()
	}
	if ret != 0 {
		return true, fmt.Errorf("failed to read events '%s', error %d", evset.go_estr, ret)
	}

	// Wait
	time.Sleep(interval)

	// Read counters
	select {
	case <-sigchan:
		ret = -1
	case e := <-watcher.Events:
		if e.Op != fsnotify.Chmod {
			ret = C.perfmon_readCounters()
		}
	default:
		ret = C.perfmon_readCounters()
	}
	if ret != 0 {
		return true, fmt.Errorf("failed to read events '%s', error %d", evset.go_estr, ret)
	}

	// Store counters
	for eidx, counter := range evset.eorder {
		gctr := C.GoString(counter)
		for _, tid := range m.cpu2tid {
			res := C.perfmon_getLastResult(gid, C.int(eidx), C.int(tid))
			fres := float64(res)
			if m.config.InvalidToZero && (math.IsNaN(fres) || math.IsInf(fres, 0)) {
				fres = 0.0
			}
			evset.results[tid][gctr] = fres
		}
	}

	// Store time in seconds the event group was measured the last time
	for _, tid := range m.cpu2tid {
		evset.results[tid]["time"] = float64(C.perfmon_getLastTimeOfGroup(gid))
	}

	// Stop counters
	select {
	case <-sigchan:
		ret = -1
	case e := <-watcher.Events:
		if e.Op != fsnotify.Chmod {
			ret = C.perfmon_stopCounters()
		}
	default:
		ret = C.perfmon_stopCounters()
	}
	if ret != 0 {
		return true, fmt.Errorf("failed to stop events '%s', error %d", evset.go_estr, ret)
	}

	// Deallocates all internal data that is used during performance monitoring
	signal.Stop(sigchan)
	select {
	case e := <-watcher.Events:
		if e.Op != fsnotify.Chmod {
			C.perfmon_finalize()
		}
	default:
		C.perfmon_finalize()
	}
	return false, nil
}

// Get all measurement results for an event set, derive the metric values out of the measurement results and send it
func (m *LikwidCollector) calcEventsetMetrics(evset LikwidEventsetConfig, interval time.Duration, output chan lp.CCMessage) error {
	invClock := float64(1.0 / m.basefreq)

	for _, tid := range m.cpu2tid {
		evset.results[tid]["inverseClock"] = invClock
		evset.results[tid]["gotime"] = interval.Seconds()
	}

	// Go over the event set metrics, derive the value out of the event:counter values and send it
	for _, metric := range m.config.Eventsets[evset.internal].Metrics {
		// The metric scope is determined in the Init() function
		// Get the map scope-id -> tids
		scopemap := m.cpu2tid
		if metric.Type == "socket" {
			scopemap = m.sock2tid
		}
		// Send all metrics with same time stamp
		// This function does only computiation, counter measurement is done before
		now := time.Now()
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
				if !math.IsNaN(value) && metric.Publish {
					y, err :=
						lp.NewMessage(
							metric.Name,
							map[string]string{
								"type": metric.Type,
							},
							m.meta,
							map[string]any{
								"value": value,
							},
							now,
						)
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

		// Send per core aggregated values
		if metric.SendCoreTotalVal {
			totalCoreValues := make(map[int]float64)
			for _, tid := range scopemap {
				if tid >= 0 && len(metric.Calc) > 0 {
					coreID := m.tid2core[tid]
					value := evset.metrics[tid][metric.Name]
					if !math.IsNaN(value) && metric.Publish {
						totalCoreValues[coreID] += value
					}
				}
			}

			for coreID, value := range totalCoreValues {
				y, err :=
					lp.NewMessage(
						metric.Name,
						map[string]string{
							"type":    "core",
							"type-id": fmt.Sprintf("%d", coreID),
						},
						m.meta,
						map[string]any{
							"value": value,
						},
						now,
					)
				if err != nil {
					continue
				}
				if len(metric.Unit) > 0 {
					y.AddMeta("unit", metric.Unit)
				}
				output <- y
			}
		}

		// Send per socket aggregated values
		if metric.SendSocketTotalVal {
			totalSocketValues := make(map[int]float64)
			for _, tid := range scopemap {
				if tid >= 0 && len(metric.Calc) > 0 {
					socketID := m.tid2socket[tid]
					value := evset.metrics[tid][metric.Name]
					if !math.IsNaN(value) && metric.Publish {
						totalSocketValues[socketID] += value
					}
				}
			}

			for socketID, value := range totalSocketValues {
				y, err :=
					lp.NewMessage(
						metric.Name,
						map[string]string{
							"type":    "socket",
							"type-id": fmt.Sprintf("%d", socketID),
						},
						m.meta,
						map[string]any{
							"value": value,
						},
						now,
					)
				if err != nil {
					continue
				}
				if len(metric.Unit) > 0 {
					y.AddMeta("unit", metric.Unit)
				}
				output <- y
			}
		}

		// Send per node aggregated value
		if metric.SendNodeTotalVal {
			var totalNodeValue float64 = 0.0
			for _, tid := range scopemap {
				if tid >= 0 && len(metric.Calc) > 0 {
					value := evset.metrics[tid][metric.Name]
					if !math.IsNaN(value) && metric.Publish {
						totalNodeValue += value
					}
				}
			}

			y, err :=
				lp.NewMessage(
					metric.Name,
					map[string]string{
						"type": "node",
					},
					m.meta,
					map[string]any{
						"value": totalNodeValue,
					},
					now,
				)
			if err != nil {
				continue
			}
			if len(metric.Unit) > 0 {
				y.AddMeta("unit", metric.Unit)
			}
			output <- y
		}
	}

	return nil
}

// Go over the global metrics, derive the value out of the event sets' metric values and send it
func (m *LikwidCollector) calcGlobalMetrics(groups []LikwidEventsetConfig, interval time.Duration, output chan lp.CCMessage) error {
	// Send all metrics with same time stamp
	// This function does only computiation, counter measurement is done before
	now := time.Now()

	for _, metric := range m.config.Metrics {
		// The metric scope is determined in the Init() function
		// Get the map scope-id -> tids
		scopemap := m.cpu2tid
		if metric.Type == "socket" {
			scopemap = m.sock2tid
		}
		for domain, tid := range scopemap {
			if tid >= 0 {
				// Here we generate parameter list
				params := make(map[string]float64)
				for _, evset := range groups {
					maps.Copy(params, evset.metrics[tid])
				}
				params["gotime"] = interval.Seconds()
				// Evaluate the metric
				value, err := agg.EvalFloat64Condition(metric.Calc, params)
				if err != nil {
					cclog.ComponentError(m.name, "Calculation for metric", metric.Name, "failed:", err.Error())
					value = 0.0
				}
				if m.config.InvalidToZero && (math.IsNaN(value) || math.IsInf(value, 0)) {
					value = 0.0
				}
				// Now we have the result, send it with the proper tags
				if !math.IsNaN(value) {
					if metric.Publish {
						y, err :=
							lp.NewMessage(
								metric.Name,
								map[string]string{
									"type": metric.Type,
								},
								m.meta,
								map[string]any{
									"value": value,
								},
								now,
							)
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

func (m *LikwidCollector) ReadThread(interval time.Duration, output chan lp.CCMessage) {
	var err error = nil
	groups := make([]LikwidEventsetConfig, 0)

	for evidx, evset := range m.config.Eventsets {
		e := genLikwidEventSet(evset)
		e.internal = evidx
		skip := false
		if !skip {
			// measure event set 'i' for 'interval' seconds
			skip, err = m.takeMeasurement(evidx, e, interval)
			if err != nil {
				cclog.ComponentError(m.name, err.Error())
				return
			}
		}

		if !skip {
			// read measurements and derive event set metrics
			err = m.calcEventsetMetrics(e, interval, output)
			if err != nil {
				cclog.ComponentError(m.name, err.Error())
				return
			}
			groups = append(groups, e)
		}
	}
	if len(groups) > 0 {
		// calculate global metrics
		err = m.calcGlobalMetrics(groups, interval, output)
		if err != nil {
			cclog.ComponentError(m.name, err.Error())
			return
		}
	}
}

// main read function taking multiple measurement rounds, each 'interval' seconds long
func (m *LikwidCollector) Read(interval time.Duration, output chan lp.CCMessage) {
	if !m.init {
		return
	}
	m.measureThread.Call(func() {
		m.ReadThread(interval, output)
	})
}

func (m *LikwidCollector) Close() {
	if m.init {
		m.init = false
		m.lock.Lock()
		m.measureThread.Terminate()
		m.initialized = false
		m.lock.Unlock()
		C.topology_finalize()
	}
}
