package main
/*
#cgo CFLAGS: -I.
#cgo LDFLAGS: -L. -llikwid -llikwid-hwloc -lm
#include <stdlib.h>
#include <likwid.h>
*/
import "C"
import "fmt"
import "unsafe"

func main() {
	var topo C.CpuTopology_t
	C.topology_init();
	topo = C.get_cpuTopology()
	cpulist := make([]C.int, topo.numHWThreads)
	for a := 0; a < int(topo.numHWThreads); a++ {
		cpulist[C.int(a)] = C.int(a)
	}
	C.perfmon_init(C.int(topo.numHWThreads), &cpulist[0])
	gstring := C.CString("INSTR_RETIRED_ANY:FIXC0")
	gid := C.perfmon_addEventSet(gstring)
	C.perfmon_setupCounters(gid)
	C.perfmon_startCounters()
	C.perfmon_stopCounters()
	v := C.perfmon_getResult(gid, 0, 0)
	fmt.Println(v)
	C.free(unsafe.Pointer(gstring))
	C.perfmon_finalize()
	C.topology_finalize();
}
