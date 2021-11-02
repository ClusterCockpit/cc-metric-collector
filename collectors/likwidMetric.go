package collectors

/*
#cgo CFLAGS: -I./likwid
#cgo LDFLAGS: -L./likwid -llikwid -llikwid-hwloc -lm
#include <stdlib.h>
#include <likwid.h>
*/
import "C"

import (
	"errors"
	"fmt"
	lp "github.com/influxdata/line-protocol"
	"log"
	"strings"
	"time"
	"unsafe"
)

type LikwidCollector struct {
	MetricCollector
	cpulist  []C.int
	sock2tid map[int]int
	metrics  map[C.int]map[string]int
	groups   map[string]C.int
}

type LikwidMetric struct {
	name         string
	search       string
	socket_scope bool
	group_idx    int
}

const GROUPPATH = `/home/unrz139/Work/cc-metric-collector/collectors/likwid/groups`

var likwid_metrics = map[string][]LikwidMetric{
	"MEM_DP": {LikwidMetric{name: "mem_bw", search: "Memory bandwidth [MBytes/s]", socket_scope: true},
		LikwidMetric{name: "pwr1", search: "Power [W]", socket_scope: true},
		LikwidMetric{name: "pwr2", search: "Power DRAM [W]", socket_scope: true},
		LikwidMetric{name: "flops_dp", search: "DP [MFLOP/s]", socket_scope: false}},
	"FLOPS_SP": {LikwidMetric{name: "clock", search: "Clock [MHz]", socket_scope: false},
		LikwidMetric{name: "cpi", search: "CPI", socket_scope: false},
		LikwidMetric{name: "flops_sp", search: "SP [MFLOP/s]", socket_scope: false}},
}

func getMetricId(group C.int, search string) (int, error) {
	for i := 0; i < int(C.perfmon_getNumberOfMetrics(group)); i++ {
		mname := C.perfmon_getMetricName(group, C.int(i))
		go_mname := C.GoString(mname)
		if strings.Contains(go_mname, search) {
			return i, nil
		}

	}
	return -1, errors.New(fmt.Sprintf("Cannot find metric for search string '%s' in group %d", search, int(group)))
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

func (m *LikwidCollector) Init() error {
	var ret C.int
	m.name = "LikwidCollector"
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
	m.metrics = make(map[C.int]map[string]int)
	m.groups = make(map[string]C.int)
	ret = C.topology_init()
	if ret != 0 {
		return errors.New("Failed to initialize LIKWID topology")
	}
	ret = C.perfmon_init(C.int(len(m.cpulist)), &m.cpulist[0])
	if ret != 0 {
		C.topology_finalize()
		return errors.New("Failed to initialize LIKWID topology")
	}
	gpath := C.CString(GROUPPATH)
	C.config_setGroupPath(gpath)
	C.free(unsafe.Pointer(gpath))

	for g, metrics := range likwid_metrics {
		cstr := C.CString(g)
		gid := C.perfmon_addEventSet(cstr)
		if gid >= 0 {
			gmetrics := 0
			for i, metric := range metrics {
				idx, err := getMetricId(gid, metric.search)
				if err != nil {
					log.Print(err)
				} else {
					likwid_metrics[g][i].group_idx = idx
					gmetrics++
				}
			}
			if gmetrics > 0 {
				m.groups[g] = gid
			}
		} else {
			log.Print("Failed to add events set ", g)
		}
		C.free(unsafe.Pointer(cstr))
	}
	if len(m.groups) == 0 {
		C.perfmon_finalize()
		C.topology_finalize()
		return errors.New("No LIKWID performance group initialized")
	}
	m.init = true
	return nil
}

func (m *LikwidCollector) Read(interval time.Duration, out *[]lp.MutableMetric) {
	if m.init {
		var ret C.int
		core_fp_any := make(map[int]float64, len(m.cpulist))
		for _, cpu := range m.cpulist {
			core_fp_any[int(cpu)] = 0.0
		}
		for gname, gid := range m.groups {
			ret = C.perfmon_setupCounters(gid)
			if ret != 0 {
				log.Print("Failed to setup performance group ", gname)
				continue
			}
			ret = C.perfmon_startCounters()
			if ret != 0 {
				log.Print("Failed to start performance group ", gname)
				continue
			}
			time.Sleep(interval)
			ret = C.perfmon_stopCounters()
			if ret != 0 {
				log.Print("Failed to stop performance group ", gname)
				continue
			}

			for _, lmetric := range likwid_metrics[gname] {
				if lmetric.name == "pwr1" || lmetric.name == "pwr2" {
					continue
				}
				if lmetric.socket_scope {
					for sid, tid := range m.sock2tid {
						res := C.perfmon_getLastMetric(gid, C.int(lmetric.group_idx), C.int(tid))
						y, err := lp.New(lmetric.name,
							map[string]string{"type": "socket", "type-id": fmt.Sprintf("%d", int(sid))},
							map[string]interface{}{"value": float64(res)},
							time.Now())
						if err == nil {
							*out = append(*out, y)
						}
						// log.Print("Metric '", lmetric.name,"' on Socket ",int(sid)," returns ", m.sockets[int(sid)][lmetric.name])
					}
				} else {
					for tid, cpu := range m.cpulist {
						res := C.perfmon_getLastMetric(gid, C.int(lmetric.group_idx), C.int(tid))
						y, err := lp.New(lmetric.name,
							map[string]string{"type": "cpu", "type-id": fmt.Sprintf("%d", int(cpu))},
							map[string]interface{}{"value": float64(res)},
							time.Now())
						if err == nil {
							*out = append(*out, y)
						}
						if lmetric.name == "flops_dp" {
							core_fp_any[int(cpu)] += 2 * float64(res)
						}
						if lmetric.name == "flops_sp" {
							core_fp_any[int(cpu)] += float64(res)
						}
						// log.Print("Metric '", lmetric.name,"' on CPU ",int(cpu)," returns ", m.cpus[int(cpu)][lmetric.name])
					}
				}
			}
			for sid, tid := range m.sock2tid {
				sum := 0.0
				valid := false
				for _, lmetric := range likwid_metrics[gname] {
					if lmetric.name == "pwr1" || lmetric.name == "pwr2" {
						res := C.perfmon_getLastMetric(gid, C.int(lmetric.group_idx), C.int(tid))
						sum += float64(res)
						valid = true
					}
				}
				if valid {
					y, err := lp.New("power",
						map[string]string{"type": "socket", "type-id": fmt.Sprintf("%d", int(sid))},
						map[string]interface{}{"value": float64(sum)},
						time.Now())
					if err == nil {
						*out = append(*out, y)
					}
				}
			}
			for cpu := range m.cpulist {
				y, err := lp.New("flops_any",
					map[string]string{"type": "cpu", "type-id": fmt.Sprintf("%d", int(cpu))},
					map[string]interface{}{"value": float64(core_fp_any[int(cpu)])},
					time.Now())
				if err == nil {
					*out = append(*out, y)
				}
			}
		}
	}
}

func (m *LikwidCollector) Close() {
	if m.init {
		C.perfmon_finalize()
		C.topology_finalize()
		m.init = false
	}
	return
}
