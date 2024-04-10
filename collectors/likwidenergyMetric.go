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
	"fmt"
	"os"
	"strings"
	"time"

	cclog "github.com/ClusterCockpit/cc-metric-collector/pkg/ccLogger"
	lp "github.com/ClusterCockpit/cc-metric-collector/pkg/ccMetric"
	topo "github.com/ClusterCockpit/cc-metric-collector/pkg/ccTopology"
	"github.com/NVIDIA/go-nvml/pkg/dl"
)

const (
	LIKWIDENERGY_LIB_NAME       = "liblikwid.so"
	LIKWIDENERGY_LIB_DL_FLAGS   = dl.RTLD_LAZY | dl.RTLD_GLOBAL
	LIKWIDENERGY_DEF_ACCESSMODE = "direct"
	LIKWIDENERGY_DEF_LOCKFILE   = "/var/run/likwid.lock"
)

// These are the fields we read from the JSON configuration
type LikwidEnergyCollectorConfig struct {
	AccessMode   string `json:"access_mode,omitempty"`
	DaemonPath   string `json:"accessdaemon_path,omitempty"`
	LibraryPath  string `json:"liblikwid_path,omitempty"`
	LockfilePath string `json:"lockfile_path,omitempty"`
	SendDiff     bool   `json:"send_difference,omitempty"`
	SendAbs      bool   `json:"send_absolute,omitempty"`
}

type LikwidEnergyDomainEntry struct {
	readcpu int
	value   uint32
	total   uint64
	tags    map[string]string
}

type LikwidEnergyDomain struct {
	values      map[int]LikwidEnergyDomainEntry
	granularity string
	metricname  string
	domaintype  int
	energyUnit  float64
}

// This contains all variables we need during execution and the variables
// defined by metricCollector (name, init, ...)
type LikwidEnergyCollector struct {
	metricCollector
	config  LikwidEnergyCollectorConfig // the configuration structure
	meta    map[string]string           // default meta information
	tags    map[string]string           // default tags
	domains map[int]LikwidEnergyDomain
}

// Init initializes the sample collector
// Called once by the collector manager
// All tags, meta data tags and metrics that do not change over the runtime should be set here
func (m *LikwidEnergyCollector) Init(config json.RawMessage) error {
	var err error = nil
	// Always set the name early in Init() to use it in cclog.Component* functions
	m.name = "LikwidEnergyCollector"
	// This is for later use, also call it early
	m.setup()
	// Tell whether the collector should be run in parallel with others (reading files, ...)
	// or it should be run serially, mostly for collectors actually doing measurements
	// because they should not measure the execution of the other collectors
	m.parallel = true
	// Define meta information sent with each metric
	// (Can also be dynamic or this is the basic set with extension through AddMeta())
	m.meta = map[string]string{"source": m.name, "group": "LIKWID", "unit": "Joules"}
	// Define tags sent with each metric
	// The 'type' tag is always needed, it defines the granularity of the metric
	// node -> whole system
	// socket -> CPU socket (requires socket ID as 'type-id' tag)
	// die -> CPU die (requires CPU die ID as 'type-id' tag)
	// memoryDomain -> NUMA domain (requires NUMA domain ID as 'type-id' tag)
	// llc -> Last level cache (requires last level cache ID as 'type-id' tag)
	// core -> single CPU core that may consist of multiple hardware threads (SMT) (requires core ID as 'type-id' tag)
	// hwthtread -> single CPU hardware thread (requires hardware thread ID as 'type-id' tag)
	// accelerator -> A accelerator device like GPU or FPGA (requires an accelerator ID as 'type-id' tag)
	m.tags = map[string]string{}
	// Read in the JSON configuration
	m.config.AccessMode = LIKWID_DEF_ACCESSMODE
	m.config.LibraryPath = LIKWID_LIB_NAME
	m.config.LockfilePath = LIKWID_DEF_LOCKFILE
	m.config.SendAbs = true
	m.config.SendDiff = true
	if len(config) > 0 {
		err = json.Unmarshal(config, &m.config)
		if err != nil {
			cclog.ComponentError(m.name, "Error reading config:", err.Error())
			return err
		}
	}
	cclog.ComponentDebug(m.name, "Opening ", m.config.LibraryPath)
	lib := dl.New(m.config.LibraryPath, LIKWID_LIB_DL_FLAGS)
	if lib == nil {
		return fmt.Errorf("error instantiating DynamicLibrary for %s", m.config.LibraryPath)
	}
	err = lib.Open()
	if err != nil {
		return fmt.Errorf("error opening %s: %v", m.config.LibraryPath, err)
	}
	cclog.ComponentDebug(m.name, "Init topology ", m.config.AccessMode)
	ret := C.topology_init()
	if ret != 0 {
		return fmt.Errorf("error initializing topology: %d", ret)
	}
	cclog.ComponentDebug(m.name, "Setting accessmode ", m.config.AccessMode)
	switch m.config.AccessMode {
	case "direct":
		C.HPMmode(0)
	case "accessdaemon":
		if len(m.config.DaemonPath) > 0 {
			p := os.Getenv("PATH")
			os.Setenv("PATH", m.config.DaemonPath+":"+p)
		}
		C.HPMmode(1)
		retCode := C.HPMinit()
		if retCode != 0 {
			err := fmt.Errorf("C.HPMinit() failed with return code %v", retCode)
			cclog.ComponentError(m.name, err.Error())
		}
	}
	initCpus := make([]int, 0)
	ret = C.HPMaddThread(0)
	if ret != 0 {
		return fmt.Errorf("error initializing access: %d", ret)
	}
	initCpus = append(initCpus, 0)

	cinfo := C.get_cpuInfo()
	domainnames := make(map[int]string)
	if cinfo.isIntel == C.int(1) {
		domainnames[0] = "pkg"
		domainnames[1] = "pp0"
		domainnames[2] = "pp1"
		domainnames[3] = "dram"
		domainnames[4] = "platform"
	} else {
		switch cinfo.family {
		case 0x17:
			domainnames[0] = "core"
			domainnames[1] = "pkg"
		case 0x19:
			switch cinfo.model {
			case 0x01, 0x21, 0x50:
				domainnames[0] = "core"
				domainnames[1] = "pkg"
			case 0x61, 0x11:
				domainnames[0] = "core"
				domainnames[1] = "l3"
			}
		}
	}

	// Set up everything that the collector requires during the Read() execution
	// Check files required, test execution of some commands, create data structure
	// for all topological entities (sockets, NUMA domains, ...)
	// Return some useful error message in case of any failures
	cclog.ComponentDebug(m.name, "Initializing Power module")
	ret = C.power_init(0)
	if ret == C.int(0) {
		cclog.ComponentPrint(m.name, "No RAPL support")
	}
	m.domains = make(map[int]LikwidEnergyDomain)
	Pinfo := C.get_powerInfo()
	for i := 0; i < int(Pinfo.numDomains); i++ {
		d := Pinfo.domains[C.int(i)]
		name := domainnames[int(d._type)]
		domain := LikwidEnergyDomain{
			values:      make(map[int]LikwidEnergyDomainEntry),
			metricname:  fmt.Sprintf("likwidenergy_%s", strings.ToLower(name)),
			granularity: "socket",
			domaintype:  int(d._type),
			energyUnit:  float64(C.power_getEnergyUnit(C.int(d._type))),
		}
		if name == "core" {
			domain.granularity = "core"
		}

		for _, c := range topo.GetTypeList(domain.granularity) {
			clist := topo.GetSocketHwthreads(c)
			if len(clist) > 0 {
				var cur C.PowerData
				if _, ok := intArrayContains(initCpus, clist[0]); !ok {
					initCpus = append(initCpus, clist[0])
					C.HPMaddThread(C.int(clist[0]))
				}
				cclog.ComponentDebug(m.name, "Reading current value on CPU ", clist[0], " for ", domain.metricname, "on", domain.granularity, c)
				ret = C.power_start(&cur, C.int(clist[0]), C.PowerType(domain.domaintype))
				cclog.ComponentDebug(m.name, "Reading ", uint64(cur.before))
				if ret == 0 {
					domain.values[c] = LikwidEnergyDomainEntry{
						readcpu: clist[0],
						value:   uint32(cur.before),
						total:   uint64(cur.before),
						tags: map[string]string{
							"type":    domain.granularity,
							"type-id": fmt.Sprintf("%d", c),
						},
					}

				}
			}
		}
		cclog.ComponentDebug(m.name, "Adding domain ", domain.metricname, " with granularity ", domain.granularity)
		m.domains[domain.domaintype] = domain
	}

	// Set this flag only if everything is initialized properly, all required files exist, ...
	m.init = true
	return err
}

// Read collects all metrics belonging to the sample collector
// and sends them through the output channel to the collector manager
func (m *LikwidEnergyCollector) Read(interval time.Duration, output chan lp.CCMetric) {
	// Create a sample metric
	timestamp := time.Now()

	for dt, domain := range m.domains {
		for i, entry := range domain.values {
			var cur C.PowerData
			ret := C.power_start(&cur, C.int(entry.readcpu), C.PowerType(dt))
			if ret == 0 {
				now := uint32(cur.before)
				diff := now - entry.value
				if now < entry.value {
					diff = (^uint32(0)) - entry.value
					diff += now
				}
				if m.config.SendDiff {
					y, err := lp.New(domain.metricname, entry.tags, m.meta, map[string]interface{}{"value": float64(diff) * domain.energyUnit}, timestamp)
					if err == nil {
						for k, v := range m.tags {
							y.AddTag(k, v)
						}
						// Send it to output channel
						output <- y
					}
				}
				if m.config.SendAbs {
					total := float64(entry.total + uint64(diff))

					y, err := lp.New(fmt.Sprintf("%s_abs", domain.metricname), entry.tags, m.meta, map[string]interface{}{"value": total * domain.energyUnit}, timestamp)
					if err == nil {
						for k, v := range m.tags {
							y.AddTag(k, v)
						}
						// Send it to output channel
						output <- y
					}
				}
				entry.value = uint32(cur.before)
				entry.total += uint64(diff)
				domain.values[i] = entry
			}
		}
	}

}

// Close metric collector: close network connection, close files, close libraries, ...
// Called once by the collector manager
func (m *LikwidEnergyCollector) Close() {
	C.power_finalize()
	C.topology_finalize()
	// Unset flag
	m.init = false
}
