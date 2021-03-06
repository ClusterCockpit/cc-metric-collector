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
	init     bool
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

func (m *LikwidCollector) Read(interval time.Duration) {
	if m.init {
		var ret C.int
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
				if lmetric.socket_scope {
					for sid, tid := range m.sock2tid {
						res := C.perfmon_getLastMetric(gid, C.int(lmetric.group_idx), C.int(tid))
						m.sockets[int(sid)][lmetric.name] = float64(res)
						// log.Print("Metric '", lmetric.name,"' on Socket ",int(sid)," returns ", m.sockets[int(sid)][lmetric.name])
					}
				} else {
					for tid, cpu := range m.cpulist {
						res := C.perfmon_getLastMetric(gid, C.int(lmetric.group_idx), C.int(tid))
						m.cpus[int(cpu)][lmetric.name] = float64(res)
						// log.Print("Metric '", lmetric.name,"' on CPU ",int(cpu)," returns ", m.cpus[int(cpu)][lmetric.name])
					}
				}
			}
			for cpu := range m.cpus {
				if flops_dp, found := m.cpus[cpu]["flops_dp"]; found {
					if flops_sp, found := m.cpus[cpu]["flops_sp"]; found {
						m.cpus[cpu]["flops_any"] = (2 * flops_dp.(float64)) + flops_sp.(float64)
					}
				}
			}
			for sid := range m.sockets {
				if pwr1, found := m.sockets[int(sid)]["pwr1"]; found {
					if pwr2, found := m.sockets[int(sid)]["pwr2"]; found {
						sum := pwr1.(float64) + pwr2.(float64)
						if sum > 0 {
							m.sockets[int(sid)]["power"] = sum
						}
						delete(m.sockets[int(sid)], "pwr2")
					}
					delete(m.sockets[int(sid)], "pwr1")
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
