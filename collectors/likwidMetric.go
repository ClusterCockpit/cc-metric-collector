package collectors

// go build -ldflags "-linkmode external -extldflags -static" bridge.go

/*
#cgo CFLAGS: -I./likwid
#cgo LDFLAGS: -L./likwid -llikwid -llikwid-hwloc -lm
#include <stdlib.h>
#include <likwid.h>
*/
import "C"

import (
	"time"
	"unsafe"

	protocol "github.com/influxdata/line-protocol"
)

type LikwidCollector struct {
	MetricCollector
	gid C.int
}

func (m *LikwidCollector) measure() (err error) {

	C.perfmon_startCounters()
	time.Sleep(10 * time.Second)
	C.perfmon_stopCounters()
	v := C.perfmon_getResult(m.gid, 0, 0)

	m.fields[0].Value = float64(v)
	return nil
}

func (m *LikwidCollector) Init() {
	m.setup()
	m.fields = make([]*protocol.Field, 1)
	m.fields[0] = &protocol.Field{Key: "instr_retired", Value: 0}

	var topo C.CpuTopology_t
	C.topology_init()
	topo = C.get_cpuTopology()
	cpulist := make([]C.int, topo.numHWThreads)

	for a := 0; a < int(topo.numHWThreads); a++ {
		cpulist[C.int(a)] = C.int(a)
	}

	C.perfmon_init(C.int(topo.numHWThreads), &cpulist[0])
	gstring := C.CString("INSTR_RETIRED_ANY:FIXC0")
	m.gid = C.perfmon_addEventSet(gstring)
	C.perfmon_setupCounters(m.gid)
	C.free(unsafe.Pointer(gstring))
}

func (m *LikwidCollector) Start(interval time.Duration) {
	m.startLoop(interval, m.measure)
}

func (c *LikwidCollector) Stop() {
	c.ticker.Stop()
	c.done <- true
	C.perfmon_finalize()
	C.topology_finalize()
}
