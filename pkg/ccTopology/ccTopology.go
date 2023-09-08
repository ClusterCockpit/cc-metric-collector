package ccTopology

import (
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

const SYSFS_CPUBASE = `/sys/devices/system/cpu`

// Structure holding all information about a hardware thread
type HwthreadEntry struct {
	CpuID              int   // CPU hardware threads
	SMT                int   // Simultaneous Multithreading ID
	ThreadSiblingsList []int // Simultaneous Multithreading siblings
	Core               int   // CPU core ID
	Socket             int   // CPU sockets (physical) ID
	Die                int   // CPU Die ID
	NumaDomain         int   // NUMA Domain
}

var cache struct {
	HwthreadList, uniqHwthreadList     []int // List of CPU hardware threads
	SMTList, uniqSMTList               []int // List of symmetric hyper threading IDs
	CoreList, uniqCoreList             []int // List of CPU core IDs
	SocketList, uniqSocketList         []int // List of CPU sockets (physical) IDs
	DieList, uniqDieList               []int // List of CPU Die IDs
	NumaDomainList, uniqNumaDomainList []int // List of NUMA Domains

	CpuData []HwthreadEntry
}

// fileToInt reads an integer value from a sysfs file
// In case of an error -1 is returned
func fileToInt(path string) int {
	buffer, err := os.ReadFile(path)
	if err != nil {
		log.Print(err)
		cclogger.ComponentError("ccTopology", "fileToInt", "Reading", path, ":", err.Error())
		return -1
	}
	stringBuffer := strings.TrimSpace(string(buffer))
	id, err := strconv.Atoi(stringBuffer)
	if err != nil {
		cclogger.ComponentError("ccTopology", "fileToInt", "Parsing", path, ":", stringBuffer, err.Error())
		return -1
	}
	return id
}

// fileToList reads a list from a sysfs file
// A list consists of value ranges separated by colon
// A range can be a single value or a range of values given by a startValue-endValue
// In case of an error nil is returned
func fileToList(path string) []int {
	// Read thread sibling list
	buffer, err := os.ReadFile(path)
	if err != nil {
		log.Print(err)
		cclogger.ComponentError("ccTopology", "fileToList", "Reading", path, ":", err.Error())
		return nil
	}

	// Create list
	list := make([]int, 0)
	stringBuffer := strings.TrimSpace(string(buffer))
	for _, valueRangeString := range strings.Split(stringBuffer, ",") {
		valueRange := strings.Split(valueRangeString, "-")
		switch len(valueRange) {
		case 1:
			singleValue, err := strconv.Atoi(valueRange[0])
			if err != nil {
				cclogger.ComponentError("CCTopology", "fileToList", "Parsing", valueRange[0], ":", err.Error())
				return nil
			}
			list = append(list, singleValue)
		case 2:
			startValue, err := strconv.Atoi(valueRange[0])
			if err != nil {
				cclogger.ComponentError("CCTopology", "fileToList", "Parsing", valueRange[0], ":", err.Error())
				return nil
			}
			endValue, err := strconv.Atoi(valueRange[1])
			if err != nil {
				cclogger.ComponentError("CCTopology", "fileToList", "Parsing", valueRange[1], ":", err.Error())
				return nil
			}
			for value := startValue; value <= endValue; value++ {
				list = append(list, value)
			}
		}
	}

	return list
}

// init initializes the cache structure
func init() {

	getHWThreads :=
		func() []int {
			globPath := filepath.Join(SYSFS_CPUBASE, "cpu[0-9]*")
			regexPath := filepath.Join(SYSFS_CPUBASE, "cpu([[:digit:]]+)")
			regex := regexp.MustCompile(regexPath)

			// File globbing for hardware threads
			files, err := filepath.Glob(globPath)
			if err != nil {
				cclogger.ComponentError("CCTopology", "init:getHWThreads", err.Error())
				return nil
			}

			hwThreadIDs := make([]int, len(files))
			for i, file := range files {
				// Extract hardware thread ID
				matches := regex.FindStringSubmatch(file)
				if len(matches) != 2 {
					cclogger.ComponentError("CCTopology", "init:getHWThreads: Failed to extract hardware thread ID from ", file)
					return nil
				}

				// Convert hardware thread ID to int
				id, err := strconv.Atoi(matches[1])
				if err != nil {
					cclogger.ComponentError("CCTopology", "init:getHWThreads: Failed to convert to int hardware thread ID ", matches[1])
					return nil
				}

				hwThreadIDs[i] = id
			}

			// Sort hardware thread IDs
			slices.Sort(hwThreadIDs)
			return hwThreadIDs
		}

	getNumaDomain :=
		func(basePath string) int {
			globPath := filepath.Join(basePath, "node*")
			regexPath := filepath.Join(basePath, "node([[:digit:]]+)")
			regex := regexp.MustCompile(regexPath)

			// File globbing for NUMA node
			files, err := filepath.Glob(globPath)
			if err != nil {
				cclogger.ComponentError("CCTopology", "init:getNumaDomain", err.Error())
				return -1
			}

			// Check, that exactly one NUMA domain was found
			if len(files) != 1 {
				cclogger.ComponentError("CCTopology", "init:getNumaDomain", "Number of NUMA domains != 1: ", len(files))
				return -1
			}

			// Extract NUMA node ID
			matches := regex.FindStringSubmatch(files[0])
			if len(matches) != 2 {
				cclogger.ComponentError("CCTopology", "init:getNumaDomain", "Failed to extract NUMA node ID from: ", files[0])
				return -1
			}

			id, err := strconv.Atoi(matches[1])
			if err != nil {
				cclogger.ComponentError("CCTopology", "init:getNumaDomain", "Failed to parse NUMA node ID from: ", matches[1])
				return -1
			}

			return id
		}

	cache.HwthreadList = getHWThreads()
	cache.CoreList = make([]int, len(cache.HwthreadList))
	cache.SocketList = make([]int, len(cache.HwthreadList))
	cache.DieList = make([]int, len(cache.HwthreadList))
	cache.SMTList = make([]int, len(cache.HwthreadList))
	cache.NumaDomainList = make([]int, len(cache.HwthreadList))
	cache.CpuData = make([]HwthreadEntry, len(cache.HwthreadList))
	for i, c := range cache.HwthreadList {
		// Set cpuBase directory for topology lookup
		cpuBase := filepath.Join(SYSFS_CPUBASE, fmt.Sprintf("cpu%d", c))
		topoBase := filepath.Join(cpuBase, "topology")

		// Lookup Core ID
		cache.CoreList[i] = fileToInt(filepath.Join(topoBase, "core_id"))

		// Lookup socket / physical package ID
		cache.SocketList[i] = fileToInt(filepath.Join(topoBase, "physical_package_id"))

		// Lookup CPU die id
		cache.DieList[i] = fileToInt(filepath.Join(topoBase, "die_id"))
		if cache.DieList[i] < 0 {
			cache.DieList[i] = cache.SocketList[i]
		}

		// Lookup thread siblings list
		threadSiblingList := fileToList(filepath.Join(topoBase, "thread_siblings_list"))

		// Find index of CPU ID in thread sibling list
		// if not found return -1
		cache.SMTList[i] = slices.Index(threadSiblingList, c)

		// Lookup NUMA domain id
		cache.NumaDomainList[i] = getNumaDomain(cpuBase)

		cache.CpuData[i] =
			HwthreadEntry{
				CpuID:              cache.HwthreadList[i],
				SMT:                cache.SMTList[i],
				ThreadSiblingsList: threadSiblingList,
				Socket:             cache.SocketList[i],
				NumaDomain:         cache.NumaDomainList[i],
				Die:                cache.DieList[i],
				Core:               cache.CoreList[i],
			}
	}

	cache.uniqHwthreadList = slices.Clone(cache.HwthreadList)
	slices.Sort(cache.uniqHwthreadList)
	cache.uniqHwthreadList = slices.Compact(cache.uniqHwthreadList)

	cache.uniqSMTList = slices.Clone(cache.SMTList)
	slices.Sort(cache.uniqSMTList)
	cache.uniqSMTList = slices.Compact(cache.uniqSMTList)

	cache.uniqCoreList = slices.Clone(cache.CoreList)
	slices.Sort(cache.uniqCoreList)
	cache.uniqCoreList = slices.Compact(cache.uniqCoreList)

	cache.uniqSocketList = slices.Clone(cache.SocketList)
	slices.Sort(cache.uniqSocketList)
	cache.uniqSocketList = slices.Compact(cache.uniqSocketList)

	cache.uniqDieList = slices.Clone(cache.DieList)
	slices.Sort(cache.uniqDieList)
	cache.uniqDieList = slices.Compact(cache.uniqDieList)

	cache.uniqNumaDomainList = slices.Clone(cache.NumaDomainList)
	slices.Sort(cache.uniqNumaDomainList)
	cache.uniqNumaDomainList = slices.Compact(cache.uniqNumaDomainList)
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
	return slices.Clone(cache.uniqNumaDomainList)
}

// DieList gets the list of CPU die IDs
func DieList() []int {
	if len(cache.uniqDieList) > 0 {
		return slices.Clone(cache.uniqDieList)
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

// CpuData returns CPU data for each hardware thread
func CpuData() []HwthreadEntry {
	// return a deep copy to protect cache data
	c := slices.Clone(cache.CpuData)
	for i := range c {
		c[i].ThreadSiblingsList = slices.Clone(cache.CpuData[i].ThreadSiblingsList)
	}
	return c
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
	return CpuInformation{
		NumNumaDomains: len(cache.uniqNumaDomainList),
		SMTWidth:       len(cache.uniqSMTList),
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
