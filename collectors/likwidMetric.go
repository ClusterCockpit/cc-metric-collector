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
	lp "github.com/influxdata/line-protocol"
	"gopkg.in/Knetic/govaluate.v2"
	"io/ioutil"
	"log"
	"math"
	"os"
	"strconv"
	"strings"
	"time"
	"unsafe"
)

type LikwidCollectorMetricConfig struct {
	Name         string `json:"name"`
	Calc         string `json:"calc"`
	Socket_scope bool   `json:"socket_scope"`
	Publish      bool   `json:"publish"`
}

type LikwidCollectorEventsetConfig struct {
	Events  map[string]string             `json:"events"`
	Metrics []LikwidCollectorMetricConfig `json:"metrics"`
}

type LikwidCollectorConfig struct {
	Eventsets      []LikwidCollectorEventsetConfig `json:"eventsets"`
	Metrics        []LikwidCollectorMetricConfig   `json:"globalmetrics"`
	ExcludeMetrics []string                        `json:"exclude_metrics"`
	ForceOverwrite bool                            `json:"force_overwrite"`
}

type LikwidCollector struct {
	MetricCollector
	cpulist   []C.int
	sock2tid  map[int]int
	metrics   map[C.int]map[string]int
	groups    []C.int
	config    LikwidCollectorConfig
	results   map[int]map[int]map[string]interface{}
	mresults  map[int]map[int]map[string]float64
	gmresults map[int]map[string]float64
	basefreq  float64
}

type LikwidMetric struct {
	name         string
	search       string
	socket_scope bool
	group_idx    int
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

func (m *LikwidCollector) Init(config []byte) error {
	var ret C.int
	m.name = "LikwidCollector"
	if len(config) > 0 {
		err := json.Unmarshal(config, &m.config)
		if err != nil {
			return err
		}
	}
	m.setup()
	cpulist := CpuList()
	m.cpulist = make([]C.int, len(cpulist))
	slist := getSocketCpus()

	m.sock2tid = make(map[int]int)
	for i, c := range cpulist {
		m.cpulist[i] = C.int(c)
		if sid, found := slist[m.cpulist[i]]; found {
			m.sock2tid[sid] = i
		}
	}
	m.results = make(map[int]map[int]map[string]interface{})
	m.mresults = make(map[int]map[int]map[string]float64)
	m.gmresults = make(map[int]map[string]float64)
	ret = C.topology_init()
	if ret != 0 {
		return errors.New("Failed to initialize LIKWID topology")
	}
	if m.config.ForceOverwrite {
		os.Setenv("LIKWID_FORCE", "1")
	}
	ret = C.perfmon_init(C.int(len(m.cpulist)), &m.cpulist[0])
	if ret != 0 {
		C.topology_finalize()
		return errors.New("Failed to initialize LIKWID topology")
	}

	for i, evset := range m.config.Eventsets {
		estr := eventsToEventStr(evset.Events)
		cstr := C.CString(estr)
		gid := C.perfmon_addEventSet(cstr)
		if gid >= 0 {
			m.groups = append(m.groups, gid)
		}
		C.free(unsafe.Pointer(cstr))
		m.results[i] = make(map[int]map[string]interface{})
		m.mresults[i] = make(map[int]map[string]float64)
		for tid, _ := range m.cpulist {
			m.results[i][tid] = make(map[string]interface{})
			m.mresults[i][tid] = make(map[string]float64)
			m.gmresults[tid] = make(map[string]float64)
		}
	}

	if len(m.groups) == 0 {
		C.perfmon_finalize()
		C.topology_finalize()
		return errors.New("No LIKWID performance group initialized")
	}
	m.basefreq = getBaseFreq()
	m.init = true
	return nil
}

func (m *LikwidCollector) Read(interval time.Duration, out *[]lp.MutableMetric) {
	if !m.init {
		return
	}
	var ret C.int

	for i, gid := range m.groups {
		evset := m.config.Eventsets[i]
		ret = C.perfmon_setupCounters(gid)
		if ret != 0 {
			log.Print("Failed to setup performance group ", C.perfmon_getGroupName(gid))
			continue
		}
		ret = C.perfmon_startCounters()
		if ret != 0 {
			log.Print("Failed to start performance group ", C.perfmon_getGroupName(gid))
			continue
		}
		time.Sleep(interval)
		ret = C.perfmon_stopCounters()
		if ret != 0 {
			log.Print("Failed to stop performance group ", C.perfmon_getGroupName(gid))
			continue
		}
		var eidx C.int
		for tid, _ := range m.cpulist {
			for eidx = 0; int(eidx) < len(evset.Events); eidx++ {
				ctr := C.perfmon_getCounterName(gid, eidx)
				gctr := C.GoString(ctr)
				res := C.perfmon_getLastResult(gid, eidx, C.int(tid))
				m.results[i][tid][gctr] = float64(res)
			}
			m.results[i][tid]["time"] = float64(interval)
			m.results[i][tid]["inverseClock"] = float64(1.0 / m.basefreq)
			for _, metric := range evset.Metrics {
				expression, err := govaluate.NewEvaluableExpression(metric.Calc)
				if err != nil {
					log.Print(err.Error())
					continue
				}
				result, err := expression.Evaluate(m.results[i][tid])
				if err != nil {
					log.Print(err.Error())
					continue
				}
				m.mresults[i][tid][metric.Name] = float64(result.(float64))
			}
		}
	}

	for _, metric := range m.config.Metrics {
		for tid, _ := range m.cpulist {
			var params map[string]interface{}
			expression, err := govaluate.NewEvaluableExpression(metric.Calc)
			if err != nil {
				log.Print(err.Error())
				continue
			}
			params = make(map[string]interface{})
			for j, _ := range m.groups {
				for mname, mres := range m.mresults[j][tid] {
					params[mname] = mres
				}
			}
			result, err := expression.Evaluate(params)
			if err != nil {
				log.Print(err.Error())
				continue
			}
			m.gmresults[tid][metric.Name] = float64(result.(float64))
		}
	}
	for i, _ := range m.groups {
		evset := m.config.Eventsets[i]
		for _, metric := range evset.Metrics {
			_, skip := stringArrayContains(m.config.ExcludeMetrics, metric.Name)
			if metric.Publish && !skip {
				if metric.Socket_scope {
					for sid, tid := range m.sock2tid {
						y, err := lp.New(metric.Name,
							map[string]string{"type": "socket", "type-id": fmt.Sprintf("%d", int(sid))},
							map[string]interface{}{"value": m.mresults[i][tid][metric.Name]},
							time.Now())
						if err == nil {
							*out = append(*out, y)
						}
					}
				} else {
					for tid, cpu := range m.cpulist {
						y, err := lp.New(metric.Name,
							map[string]string{"type": "cpu", "type-id": fmt.Sprintf("%d", int(cpu))},
							map[string]interface{}{"value": m.mresults[i][tid][metric.Name]},
							time.Now())
						if err == nil {
							*out = append(*out, y)
						}
					}
				}
			}
		}
	}
	for _, metric := range m.config.Metrics {
		_, skip := stringArrayContains(m.config.ExcludeMetrics, metric.Name)
		if metric.Publish && !skip {
			if metric.Socket_scope {
				for sid, tid := range m.sock2tid {
					y, err := lp.New(metric.Name,
						map[string]string{"type": "socket", "type-id": fmt.Sprintf("%d", int(sid))},
						map[string]interface{}{"value": m.gmresults[tid][metric.Name]},
						time.Now())
					if err == nil {
						*out = append(*out, y)
					}
				}
			} else {
				for tid, cpu := range m.cpulist {
					y, err := lp.New(metric.Name,
						map[string]string{"type": "cpu", "type-id": fmt.Sprintf("%d", int(cpu))},
						map[string]interface{}{"value": m.gmresults[tid][metric.Name]},
						time.Now())
					if err == nil {
						*out = append(*out, y)
					}
				}
			}
		}
	}
}

func (m *LikwidCollector) Close() {
	if m.init {
		m.init = false
		C.perfmon_finalize()
		C.topology_finalize()
	}
	return
}
