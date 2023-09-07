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

// Structure holding all information about a hardware thread
type HwthreadEntry struct {
	CpuID      int
	SMT        int
	Core       int
	Socket     int
	NumaDomain int
	Die        int
}

var cache struct {
	HwthreadList     []int
	uniqHwthreadList []int

	CoreList     []int
	uniqCoreList []int

	SocketList     []int
	uniqSocketList []int

	DieList     []int
	uniqDieList []int

	CpuData []HwthreadEntry
}

func init() {
	file, err := os.Open(PROCFS_CPUINFO)
	if err != nil {
		log.Print(err)
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lineSplit := strings.Split(scanner.Text(), ":")
		if len(lineSplit) == 2 {
			key := strings.TrimSpace(lineSplit[0])
			value := strings.TrimSpace(lineSplit[1])
			switch key {
			case "physical id":
				id, err := strconv.Atoi(value)
				if err != nil {
					log.Print(err)
					return
				}
				cache.SocketList = append(cache.SocketList, id)
			case "processor":
				id, err := strconv.Atoi(value)
				if err != nil {
					log.Print(err)
					return
				}
				cache.HwthreadList = append(cache.HwthreadList, id)
			case "core id":
				id, err := strconv.Atoi(value)
				if err != nil {
					log.Print(err)
					return
				}
				cache.CoreList = append(cache.CoreList, id)
			}
		}
	}

	cache.DieList = make([]int, len(cache.HwthreadList))
	for i, c := range cache.HwthreadList {
		// Set base directory for topology lookup
		cpuStr := fmt.Sprintf("cpu%d", c)
		base := filepath.Join("/sys/devices/system/cpu", cpuStr)
		topoBase := filepath.Join(base, "topology")

		// Lookup CPU die id
		cache.DieList[i] = fileToInt(filepath.Join(topoBase, "die_id"))
		if cache.DieList[i] < 0 {
			cache.DieList[i] = cache.SocketList[i]
		}
	}

	cache.uniqHwthreadList = slices.Clone(cache.HwthreadList)
	slices.Sort(cache.uniqHwthreadList)
	cache.uniqHwthreadList = slices.Compact(cache.uniqHwthreadList)

	cache.uniqCoreList = slices.Clone(cache.CoreList)
	slices.Sort(cache.uniqCoreList)
	cache.uniqCoreList = slices.Compact(cache.uniqCoreList)

	cache.uniqSocketList = slices.Clone(cache.SocketList)
	slices.Sort(cache.uniqSocketList)
	cache.uniqSocketList = slices.Compact(cache.uniqSocketList)

	cache.uniqDieList = slices.Clone(cache.DieList)
	slices.Sort(cache.uniqDieList)
	cache.uniqDieList = slices.Compact(cache.uniqDieList)

	getSMT :=
		func(cpuID int, basePath string) int {
			buffer, err := os.ReadFile(filepath.Join(basePath, "thread_siblings_list"))
			if err != nil {
				cclogger.ComponentError("CCTopology", "CpuData:getSMT", err.Error())
			}
			threadList := make([]int, 0)
			stringBuffer := strings.TrimSpace(string(buffer))
			for _, x := range strings.Split(stringBuffer, ",") {
				id, err := strconv.Atoi(x)
				if err != nil {
					cclogger.ComponentError("CCTopology", "CpuData:getSMT", err.Error())
				}
				threadList = append(threadList, id)
			}
			if i := slices.Index(threadList, cpuID); i != -1 {
				return i
			}
			return 1
		}

	getNumaDomain :=
		func(basePath string) int {
			globPath := filepath.Join(basePath, "node*")
			regexPath := filepath.Join(basePath, "node([[:digit:]]+)")
			regex := regexp.MustCompile(regexPath)
			files, err := filepath.Glob(globPath)
			if err != nil {
				cclogger.ComponentError("CCTopology", "CpuData:getNumaDomain", err.Error())
			}
			for _, file := range files {
				matches := regex.FindStringSubmatch(file)
				if len(matches) == 2 {
					id, err := strconv.Atoi(matches[1])
					if err == nil {
						return id
					}
				}
			}
			return 0
		}

	cache.CpuData = make([]HwthreadEntry, len(cache.HwthreadList))
	for i := range cache.HwthreadList {
		cache.CpuData[i] =
			HwthreadEntry{
				CpuID:      cache.HwthreadList[i],
				Socket:     cache.SocketList[i],
				NumaDomain: -1,
				Die:        cache.DieList[i],
				Core:       cache.CoreList[i],
			}
	}
	for i := range cache.CpuData {
		cEntry := &cache.CpuData[i]

		// Set base directory for topology lookup
		cpuStr := fmt.Sprintf("cpu%d", cEntry.CpuID)
		base := filepath.Join("/sys/devices/system/cpu", cpuStr)
		topoBase := filepath.Join(base, "topology")

		// Lookup SMT thread id
		cEntry.SMT = getSMT(cEntry.CpuID, topoBase)

		// Lookup NUMA domain id
		cEntry.NumaDomain = getNumaDomain(base)
	}
}

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
	stringBuffer := strings.TrimSpace(string(buffer))
	id, err := strconv.Atoi(stringBuffer)
	if err != nil {
		cclogger.ComponentError("ccTopology", "Parsing", path, ":", stringBuffer, err.Error())
		return -1
	}
	return id
}

// SocketList gets the list of CPU socket IDs
func SocketList() []int {
	return slices.Clone(cache.uniqSocketList)
}

// HwthreadList gets the list of hardware thread IDs in the order of listing in /proc/cpuinfo
func HwthreadList() []int {
	return slices.Clone(cache.uniqHwthreadList)
}

// Get list of hardware thread IDs in the order of listing in /proc/cpuinfo
// Deprecated! Use HwthreadList()
func CpuList() []int {
	return HwthreadList()
}

// CoreList gets the list of CPU core IDs in the order of listing in /proc/cpuinfo
func CoreList() []int {
	return slices.Clone(cache.uniqCoreList)
}

// Get list of NUMA node IDs
func NumaNodeList() []int {
	numaList := make([]int, 0)
	globPath := filepath.Join(SYSFS_NUMABASE, "node*")
	regexPath := filepath.Join(SYSFS_NUMABASE, "node(\\d+)")
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

// DieList gets the list of CPU die IDs
func DieList() []int {
	cpuList := HwthreadList()
	dieList := make([]int, 0)
	for _, c := range cpuList {
		diePath := filepath.Join(
			SYSFS_CPUBASE,
			fmt.Sprintf("cpu%d", c),
			"topology/die_id")
		dieID := fileToInt(diePath)
		if dieID > 0 {
			if found := slices.Contains(dieList, dieID); !found {
				dieList = append(dieList, dieID)
			}
		}
	}
	if len(dieList) > 0 {
		return dieList
	}
	return SocketList()
}

// GetTypeList gets the list of specified type using the naming format inside ClusterCockpit
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

func CpuData() []HwthreadEntry {
	return slices.Clone(cache.CpuData)
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

// CpuInformation reports basic information about the CPU
func CpuInfo() CpuInformation {

	smtList := make([]int, 0)
	numaList := make([]int, 0)
	for i := range cache.CpuData {
		d := &cache.CpuData[i]
		if ok := slices.Contains(smtList, d.SMT); !ok {
			smtList = append(smtList, d.SMT)
		}
		if ok := slices.Contains(numaList, d.NumaDomain); !ok {
			numaList = append(numaList, d.NumaDomain)
		}
	}
	return CpuInformation{
		NumNumaDomains: len(numaList),
		SMTWidth:       len(smtList),
		NumDies:        len(cache.uniqDieList),
		NumCores:       len(cache.uniqCoreList),
		NumSockets:     len(cache.uniqSocketList),
		NumHWthreads:   len(cache.uniqHwthreadList),
	}
}

// GetHwthreadSocket gets the CPU socket ID for a given hardware thread ID
// In case hardware thread ID is not found -1 is returned
func GetHwthreadSocket(cpuID int) int {
	for i := range cache.CpuData {
		d := &cache.CpuData[i]
		if d.CpuID == cpuID {
			return d.Socket
		}
	}
	return -1
}

// GetHwthreadNumaDomain gets the NUMA domain ID for a given hardware thread ID
// In case hardware thread ID is not found -1 is returned
func GetHwthreadNumaDomain(cpuID int) int {
	for i := range cache.CpuData {
		d := &cache.CpuData[i]
		if d.CpuID == cpuID {
			return d.NumaDomain
		}
	}
	return -1
}

// GetHwthreadDie gets the CPU die ID for a given hardware thread ID
// In case hardware thread ID is not found -1 is returned
func GetHwthreadDie(cpuID int) int {
	for i := range cache.CpuData {
		d := &cache.CpuData[i]
		if d.CpuID == cpuID {
			return d.Die
		}
	}
	return -1
}

// GetHwthreadCore gets the CPU core ID for a given hardware thread ID
// In case hardware thread ID is not found -1 is returned
func GetHwthreadCore(cpuID int) int {
	for i := range cache.CpuData {
		d := &cache.CpuData[i]
		if d.CpuID == cpuID {
			return d.Core
		}
	}
	return -1
}

// GetSocketHwthreads gets all hardware thread IDs associated with a CPU socket
func GetSocketHwthreads(socket int) []int {
	cpuList := make([]int, 0)
	for i := range cache.CpuData {
		d := &cache.CpuData[i]
		if d.Socket == socket {
			cpuList = append(cpuList, d.CpuID)
		}
	}
	return cpuList
}

// GetNumaDomainHwthreads gets the all hardware thread IDs associated with a NUMA domain
func GetNumaDomainHwthreads(numaDomain int) []int {
	cpuList := make([]int, 0)
	for i := range cache.CpuData {
		d := &cache.CpuData[i]
		if d.NumaDomain == numaDomain {
			cpuList = append(cpuList, d.CpuID)
		}
	}
	return cpuList
}

// GetDieHwthreads gets all hardware thread IDs associated with a CPU die
func GetDieHwthreads(die int) []int {
	cpuList := make([]int, 0)
	for i := range cache.CpuData {
		d := &cache.CpuData[i]
		if d.Die == die {
			cpuList = append(cpuList, d.CpuID)
		}
	}
	return cpuList
}

// GetCoreHwthreads get all hardware thread IDs associated with a CPU core
func GetCoreHwthreads(core int) []int {
	cpuList := make([]int, 0)
	for i := range cache.CpuData {
		d := &cache.CpuData[i]
		if d.Core == core {
			cpuList = append(cpuList, d.CpuID)
		}
	}
	return cpuList
}
