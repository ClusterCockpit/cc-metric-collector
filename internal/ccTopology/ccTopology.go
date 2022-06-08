package ccTopology

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	cclogger "github.com/ClusterCockpit/cc-metric-collector/internal/ccLogger"
)

const SYSFS_NUMABASE = `/sys/devices/system/node`
const SYSFS_CPUBASE = `/sys/devices/system/cpu`
const PROCFS_CPUINFO = `/proc/cpuinfo`

// intArrayContains scans an array of ints if the value str is present in the array
// If the specified value is found, the corresponding array index is returned.
// The bool value is used to signal success or failure
func intArrayContains(array []int, str int) (int, bool) {
	for i, a := range array {
		if a == str {
			return i, true
		}
	}
	return -1, false
}

// Used internally for sysfs file reads
func fileToInt(path string) int {
	buffer, err := ioutil.ReadFile(path)
	if err != nil {
		log.Print(err)
		cclogger.ComponentError("ccTopology", "Reading", path, ":", err.Error())
		return -1
	}
	sbuffer := strings.Replace(string(buffer), "\n", "", -1)
	var id int64
	//_, err = fmt.Scanf("%d", sbuffer, &id)
	id, err = strconv.ParseInt(sbuffer, 10, 32)
	if err != nil {
		cclogger.ComponentError("ccTopology", "Parsing", path, ":", sbuffer, err.Error())
		return -1
	}
	return int(id)
}

// Get list of CPU socket IDs
func SocketList() []int {
	buffer, err := ioutil.ReadFile(string(PROCFS_CPUINFO))
	if err != nil {
		log.Print(err)
		return nil
	}
	ll := strings.Split(string(buffer), "\n")
	packs := make([]int, 0)
	for _, line := range ll {
		if strings.HasPrefix(line, "physical id") {
			lv := strings.Fields(line)
			id, err := strconv.ParseInt(lv[3], 10, 32)
			if err != nil {
				log.Print(err)
				return packs
			}
			_, found := intArrayContains(packs, int(id))
			if !found {
				packs = append(packs, int(id))
			}
		}
	}
	return packs
}

// Get list of hardware thread IDs in the order of listing in /proc/cpuinfo
func HwthreadList() []int {
	buffer, err := ioutil.ReadFile(string(PROCFS_CPUINFO))
	if err != nil {
		log.Print(err)
		return nil
	}
	ll := strings.Split(string(buffer), "\n")
	cpulist := make([]int, 0)
	for _, line := range ll {
		if strings.HasPrefix(line, "processor") {
			lv := strings.Fields(line)
			id, err := strconv.ParseInt(lv[2], 10, 32)
			if err != nil {
				log.Print(err)
				return cpulist
			}
			_, found := intArrayContains(cpulist, int(id))
			if !found {
				cpulist = append(cpulist, int(id))
			}
		}
	}
	return cpulist
}

// Get list of hardware thread IDs in the order of listing in /proc/cpuinfo
// Deprecated! Use HwthreadList()
func CpuList() []int {
	return HwthreadList()
}

// Get list of CPU core IDs in the order of listing in /proc/cpuinfo
func CoreList() []int {
	buffer, err := ioutil.ReadFile(string(PROCFS_CPUINFO))
	if err != nil {
		log.Print(err)
		return nil
	}
	ll := strings.Split(string(buffer), "\n")
	corelist := make([]int, 0)
	for _, line := range ll {
		if strings.HasPrefix(line, "core id") {
			lv := strings.Fields(line)
			id, err := strconv.ParseInt(lv[3], 10, 32)
			if err != nil {
				log.Print(err)
				return corelist
			}
			_, found := intArrayContains(corelist, int(id))
			if !found {
				corelist = append(corelist, int(id))
			}
		}
	}
	return corelist
}

// Get list of NUMA node IDs
func NumaNodeList() []int {
	numaList := make([]int, 0)
	globPath := filepath.Join(string(SYSFS_NUMABASE), "node*")
	regexPath := filepath.Join(string(SYSFS_NUMABASE), "node(\\d+)")
	regex := regexp.MustCompile(regexPath)
	files, err := filepath.Glob(globPath)
	if err != nil {
		cclogger.ComponentError("CCTopology", "NumaNodeList", err.Error())
	}
	for _, f := range files {
		if !regex.MatchString(f) {
			continue
		}
		finfo, err := os.Lstat(f)
		if err != nil {
			continue
		}
		if !finfo.IsDir() {
			continue
		}
		matches := regex.FindStringSubmatch(f)
		if len(matches) == 2 {
			id, err := strconv.Atoi(matches[1])
			if err == nil {
				if _, found := intArrayContains(numaList, id); !found {
					numaList = append(numaList, id)
				}
			}
		}

	}
	return numaList
}

// Get list of CPU die IDs
func DieList() []int {
	cpulist := HwthreadList()
	dielist := make([]int, 0)
	for _, c := range cpulist {
		diepath := filepath.Join(string(SYSFS_CPUBASE), fmt.Sprintf("cpu%d", c), "topology/die_id")
		dieid := fileToInt(diepath)
		if dieid > 0 {
			_, found := intArrayContains(dielist, int(dieid))
			if !found {
				dielist = append(dielist, int(dieid))
			}
		}
	}
	if len(dielist) > 0 {
		return dielist
	}
	return SocketList()
}

// Get list of specified type using the naming format inside ClusterCockpit
func GetTypeList(topology_type string) []int {
	switch topology_type {
	case "node":
		return []int{0}
	case "socket":
		return SocketList()
	case "die":
		return DieList()
	case "memoryDomain":
		return NumaNodeList()
	case "core":
		return CoreList()
	case "hwthread":
		return HwthreadList()
	}
	return []int{}
}

// Structure holding all information about a hardware thread
type HwthreadEntry struct {
	Cpuid      int
	SMT        int
	Core       int
	Socket     int
	Numadomain int
	Die        int
}

func CpuData() []HwthreadEntry {

	// fileToInt := func(path string) int {
	// 	buffer, err := ioutil.ReadFile(path)
	// 	if err != nil {
	// 		log.Print(err)
	// 		//cclogger.ComponentError("ccTopology", "Reading", path, ":", err.Error())
	// 		return -1
	// 	}
	// 	sbuffer := strings.Replace(string(buffer), "\n", "", -1)
	// 	var id int64
	// 	//_, err = fmt.Scanf("%d", sbuffer, &id)
	// 	id, err = strconv.ParseInt(sbuffer, 10, 32)
	// 	if err != nil {
	// 		cclogger.ComponentError("ccTopology", "Parsing", path, ":", sbuffer, err.Error())
	// 		return -1
	// 	}
	// 	return int(id)
	// }
	getCore := func(basepath string) int {
		return fileToInt(fmt.Sprintf("%s/core_id", basepath))
	}

	getSocket := func(basepath string) int {
		return fileToInt(fmt.Sprintf("%s/physical_package_id", basepath))
	}

	getDie := func(basepath string) int {
		return fileToInt(fmt.Sprintf("%s/die_id", basepath))
	}

	getSMT := func(cpuid int, basepath string) int {
		buffer, err := ioutil.ReadFile(fmt.Sprintf("%s/thread_siblings_list", basepath))
		if err != nil {
			cclogger.ComponentError("CCTopology", "CpuData:getSMT", err.Error())
		}
		threadlist := make([]int, 0)
		sbuffer := strings.Replace(string(buffer), "\n", "", -1)
		for _, x := range strings.Split(sbuffer, ",") {
			id, err := strconv.ParseInt(x, 10, 32)
			if err != nil {
				cclogger.ComponentError("CCTopology", "CpuData:getSMT", err.Error())
			}
			threadlist = append(threadlist, int(id))
		}
		for i, x := range threadlist {
			if x == cpuid {
				return i
			}
		}
		return 1
	}

	getNumaDomain := func(basepath string) int {
		globPath := filepath.Join(basepath, "node*")
		regexPath := filepath.Join(basepath, "node(\\d+)")
		regex := regexp.MustCompile(regexPath)
		files, err := filepath.Glob(globPath)
		if err != nil {
			cclogger.ComponentError("CCTopology", "CpuData:getNumaDomain", err.Error())
		}
		for _, f := range files {
			finfo, err := os.Lstat(f)
			if err == nil && finfo.IsDir() {
				matches := regex.FindStringSubmatch(f)
				if len(matches) == 2 {
					id, err := strconv.Atoi(matches[1])
					if err == nil {
						return id
					}
				}
			}
		}
		return 0
	}

	clist := make([]HwthreadEntry, 0)
	for _, c := range HwthreadList() {
		clist = append(clist, HwthreadEntry{Cpuid: c})
	}
	for i, centry := range clist {
		centry.Socket = -1
		centry.Numadomain = -1
		centry.Die = -1
		centry.Core = -1
		// Set base directory for topology lookup
		cpustr := fmt.Sprintf("cpu%d", centry.Cpuid)
		base := filepath.Join("/sys/devices/system/cpu", cpustr)
		topoBase := filepath.Join(base, "topology")

		// Lookup CPU core id
		centry.Core = getCore(topoBase)

		// Lookup CPU socket id
		centry.Socket = getSocket(topoBase)

		// Lookup CPU die id
		centry.Die = getDie(topoBase)
		if centry.Die < 0 {
			centry.Die = centry.Socket
		}

		// Lookup SMT thread id
		centry.SMT = getSMT(centry.Cpuid, topoBase)

		// Lookup NUMA domain id
		centry.Numadomain = getNumaDomain(base)

		// Update values in output list
		clist[i] = centry
	}
	return clist
}

// Structure holding basic information about a CPU
type CpuInformation struct {
	NumHWthreads   int
	SMTWidth       int
	NumSockets     int
	NumDies        int
	NumCores       int
	NumNumaDomains int
}

// Get basic information about the CPU
func CpuInfo() CpuInformation {
	var c CpuInformation

	smtList := make([]int, 0)
	numaList := make([]int, 0)
	dieList := make([]int, 0)
	socketList := make([]int, 0)
	coreList := make([]int, 0)
	cdata := CpuData()
	for _, d := range cdata {
		if _, ok := intArrayContains(smtList, d.SMT); !ok {
			smtList = append(smtList, d.SMT)
		}
		if _, ok := intArrayContains(numaList, d.Numadomain); !ok {
			numaList = append(numaList, d.Numadomain)
		}
		if _, ok := intArrayContains(dieList, d.Die); !ok {
			dieList = append(dieList, d.Die)
		}
		if _, ok := intArrayContains(socketList, d.Socket); !ok {
			socketList = append(socketList, d.Socket)
		}
		if _, ok := intArrayContains(coreList, d.Core); !ok {
			coreList = append(coreList, d.Core)
		}
	}
	c.NumNumaDomains = len(numaList)
	c.SMTWidth = len(smtList)
	c.NumDies = len(dieList)
	c.NumCores = len(coreList)
	c.NumSockets = len(socketList)
	c.NumHWthreads = len(cdata)
	return c
}

// Get the CPU socket ID for a given hardware thread ID
func GetHwthreadSocket(cpuid int) int {
	cdata := CpuData()
	for _, d := range cdata {
		if d.Cpuid == cpuid {
			return d.Socket
		}
	}
	return -1
}

// Get the NUMA node ID for a given hardware thread ID
func GetHwthreadNumaDomain(cpuid int) int {
	cdata := CpuData()
	for _, d := range cdata {
		if d.Cpuid == cpuid {
			return d.Numadomain
		}
	}
	return -1
}

// Get the CPU die ID for a given hardware thread ID
func GetHwthreadDie(cpuid int) int {
	cdata := CpuData()
	for _, d := range cdata {
		if d.Cpuid == cpuid {
			return d.Die
		}
	}
	return -1
}

// Get the CPU core ID for a given hardware thread ID
func GetHwthreadCore(cpuid int) int {
	cdata := CpuData()
	for _, d := range cdata {
		if d.Cpuid == cpuid {
			return d.Core
		}
	}
	return -1
}

// Get the all hardware thread ID associated with a CPU socket
func GetSocketHwthreads(socket int) []int {
	all := CpuData()
	cpulist := make([]int, 0)
	for _, d := range all {
		if d.Socket == socket {
			cpulist = append(cpulist, d.Cpuid)
		}
	}
	return cpulist
}

// Get the all hardware thread ID associated with a NUMA node
func GetNumaDomainHwthreads(domain int) []int {
	all := CpuData()
	cpulist := make([]int, 0)
	for _, d := range all {
		if d.Numadomain == domain {
			cpulist = append(cpulist, d.Cpuid)
		}
	}
	return cpulist
}

// Get the all hardware thread ID associated with a CPU die
func GetDieHwthreads(die int) []int {
	all := CpuData()
	cpulist := make([]int, 0)
	for _, d := range all {
		if d.Die == die {
			cpulist = append(cpulist, d.Cpuid)
		}
	}
	return cpulist
}

// Get the all hardware thread ID associated with a CPU core
func GetCoreHwthreads(core int) []int {
	all := CpuData()
	cpulist := make([]int, 0)
	for _, d := range all {
		if d.Core == core {
			cpulist = append(cpulist, d.Cpuid)
		}
	}
	return cpulist
}
