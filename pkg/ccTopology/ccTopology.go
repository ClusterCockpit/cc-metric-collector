package ccTopology

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	cclogger "github.com/ClusterCockpit/cc-metric-collector/pkg/ccLogger"
	"golang.org/x/exp/slices"
)

const (
	SYSFS_NUMABASE = `/sys/devices/system/node`
	SYSFS_CPUBASE  = `/sys/devices/system/cpu`
	PROCFS_CPUINFO = `/proc/cpuinfo`
)

// fileToInt reads an integer value from a file
// In case of an error -1 is returned
// Used internally for sysfs file reads
func fileToInt(path string) int {
	buffer, err := os.ReadFile(path)
	if err != nil {
		log.Print(err)
		cclogger.ComponentError("ccTopology", "Reading", path, ":", err.Error())
		return -1
	}
	sbuffer := strings.TrimSpace(string(buffer))
	id, err := strconv.ParseInt(sbuffer, 10, 32)
	if err != nil {
		cclogger.ComponentError("ccTopology", "Parsing", path, ":", sbuffer, err.Error())
		return -1
	}
	return int(id)
}

// SocketList gets the list of CPU socket IDs
func SocketList() []int {

	file, err := os.Open(string(PROCFS_CPUINFO))
	if err != nil {
		log.Print(err)
		return nil
	}
	defer file.Close()

	packs := make([]int, 0)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "physical id") {
			lv := strings.Fields(line)
			id, err := strconv.ParseInt(lv[3], 10, 32)
			if err != nil {
				log.Print(err)
				return nil
			}
			if found := slices.Contains(packs, int(id)); !found {
				packs = append(packs, int(id))
			}
		}
	}
	return packs
}

// HwthreadList gets the list of hardware thread IDs in the order of listing in /proc/cpuinfo
func HwthreadList() []int {
	file, err := os.Open(string(PROCFS_CPUINFO))
	if err != nil {
		log.Print(err)
		return nil
	}
	defer file.Close()

	cpuList := make([]int, 0)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "processor") {
			lv := strings.Fields(line)
			id, err := strconv.ParseInt(lv[2], 10, 32)
			if err != nil {
				log.Print(err)
				return nil
			}
			if found := slices.Contains(cpuList, int(id)); !found {
				cpuList = append(cpuList, int(id))
			}
		}
	}
	return cpuList
}

// Get list of hardware thread IDs in the order of listing in /proc/cpuinfo
// Deprecated! Use HwthreadList()
func CpuList() []int {
	return HwthreadList()
}

// CoreList gets the list of CPU core IDs in the order of listing in /proc/cpuinfo
func CoreList() []int {
	file, err := os.Open(string(PROCFS_CPUINFO))
	if err != nil {
		log.Print(err)
		return nil
	}
	defer file.Close()

	coreList := make([]int, 0)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "core id") {
			lv := strings.Fields(line)
			id, err := strconv.ParseInt(lv[3], 10, 32)
			if err != nil {
				log.Print(err)
				return nil
			}
			if found := slices.Contains(coreList, int(id)); !found {
				coreList = append(coreList, int(id))
			}
		}
	}
	return coreList
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
				if found := slices.Contains(numaList, id); !found {
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
			if found := slices.Contains(dielist, int(dieid)); !found {
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
	// 	buffer, err := os.ReadFile(path)
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
		buffer, err := os.ReadFile(fmt.Sprintf("%s/thread_siblings_list", basepath))
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

	smtList := make([]int, 0)
	numaList := make([]int, 0)
	dieList := make([]int, 0)
	socketList := make([]int, 0)
	coreList := make([]int, 0)
	cdata := CpuData()
	for _, d := range cdata {
		if ok := slices.Contains(smtList, d.SMT); !ok {
			smtList = append(smtList, d.SMT)
		}
		if ok := slices.Contains(numaList, d.Numadomain); !ok {
			numaList = append(numaList, d.Numadomain)
		}
		if ok := slices.Contains(dieList, d.Die); !ok {
			dieList = append(dieList, d.Die)
		}
		if ok := slices.Contains(socketList, d.Socket); !ok {
			socketList = append(socketList, d.Socket)
		}
		if ok := slices.Contains(coreList, d.Core); !ok {
			coreList = append(coreList, d.Core)
		}
	}
	return CpuInformation{
		NumNumaDomains: len(numaList),
		SMTWidth:       len(smtList),
		NumDies:        len(dieList),
		NumCores:       len(coreList),
		NumSockets:     len(socketList),
		NumHWthreads:   len(cdata),
	}
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
