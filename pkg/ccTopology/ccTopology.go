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
	SocketList   []int
	HwthreadList []int
	CoreList     []int
	CpuData      []HwthreadEntry
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

func initSocketHwthreadCoreList() {
	file, err := os.Open(string(PROCFS_CPUINFO))
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
				if found := slices.Contains(cache.SocketList, id); !found {
					cache.SocketList = append(cache.SocketList, id)
				}
			case "processor":
				id, err := strconv.Atoi(value)
				if err != nil {
					log.Print(err)
					return
				}
				if found := slices.Contains(cache.HwthreadList, id); !found {
					cache.HwthreadList = append(cache.HwthreadList, id)
				}
			case "core id":
				id, err := strconv.Atoi(value)
				if err != nil {
					log.Print(err)
					return
				}
				if found := slices.Contains(cache.CoreList, id); !found {
					cache.CoreList = append(cache.CoreList, id)
				}
			}
		}
	}
}

// SocketList gets the list of CPU socket IDs
func SocketList() []int {
	if cache.SocketList == nil {
		initSocketHwthreadCoreList()
	}
	return slices.Clone(cache.SocketList)
}

// HwthreadList gets the list of hardware thread IDs in the order of listing in /proc/cpuinfo
func HwthreadList() []int {
	if cache.HwthreadList == nil {
		initSocketHwthreadCoreList()
	}
	return slices.Clone(cache.HwthreadList)
}

// Get list of hardware thread IDs in the order of listing in /proc/cpuinfo
// Deprecated! Use HwthreadList()
func CpuList() []int {
	return HwthreadList()
}

// CoreList gets the list of CPU core IDs in the order of listing in /proc/cpuinfo
func CoreList() []int {
	if cache.CoreList == nil {
		initSocketHwthreadCoreList()
	}
	return slices.Clone(cache.CoreList)
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

// DieList gets the list of CPU die IDs
func DieList() []int {
	cpuList := HwthreadList()
	dieList := make([]int, 0)
	for _, c := range cpuList {
		diePath := filepath.Join(
			string(SYSFS_CPUBASE),
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

func initCpuData() {

	getCore :=
		func(basePath string) int {
			return fileToInt(filepath.Join(basePath, "core_id"))
		}

	getSocket :=
		func(basePath string) int {
			return fileToInt(filepath.Join(basePath, "physical_package_id"))
		}

	getDie :=
		func(basePath string) int {
			return fileToInt(filepath.Join(basePath, "die_id"))
		}

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

	for _, c := range HwthreadList() {
		cache.CpuData = append(cache.CpuData,
			HwthreadEntry{
				CpuID:      c,
				Socket:     -1,
				NumaDomain: -1,
				Die:        -1,
				Core:       -1,
			})
	}
	for i := range cache.CpuData {
		cEntry := &cache.CpuData[i]

		// Set base directory for topology lookup
		cpuStr := fmt.Sprintf("cpu%d", cEntry.CpuID)
		base := filepath.Join("/sys/devices/system/cpu", cpuStr)
		topoBase := filepath.Join(base, "topology")

		// Lookup CPU core id
		cEntry.Core = getCore(topoBase)

		// Lookup CPU socket id
		cEntry.Socket = getSocket(topoBase)

		// Lookup CPU die id
		cEntry.Die = getDie(topoBase)
		if cEntry.Die < 0 {
			cEntry.Die = cEntry.Socket
		}

		// Lookup SMT thread id
		cEntry.SMT = getSMT(cEntry.CpuID, topoBase)

		// Lookup NUMA domain id
		cEntry.NumaDomain = getNumaDomain(base)
	}
}

func CpuData() []HwthreadEntry {
	if cache.CpuData == nil {
		initCpuData()
	}
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
	dieList := make([]int, 0)
	socketList := make([]int, 0)
	coreList := make([]int, 0)
	cpuData := CpuData()
	for i := range cpuData {
		d := &cpuData[i]
		if ok := slices.Contains(smtList, d.SMT); !ok {
			smtList = append(smtList, d.SMT)
		}
		if ok := slices.Contains(numaList, d.NumaDomain); !ok {
			numaList = append(numaList, d.NumaDomain)
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
		NumHWthreads:   len(cpuData),
	}
}

// GetHwthreadSocket gets the CPU socket ID for a given hardware thread ID
// In case hardware thread ID is not found -1 is returned
func GetHwthreadSocket(cpuID int) int {
	cpuData := CpuData()
	for i := range cpuData {
		d := &cpuData[i]
		if d.CpuID == cpuID {
			return d.Socket
		}
	}
	return -1
}

// GetHwthreadNumaDomain gets the NUMA domain ID for a given hardware thread ID
// In case hardware thread ID is not found -1 is returned
func GetHwthreadNumaDomain(cpuID int) int {
	cpuData := CpuData()
	for i := range cpuData {
		d := &cpuData[i]
		if d.CpuID == cpuID {
			return d.NumaDomain
		}
	}
	return -1
}

// GetHwthreadDie gets the CPU die ID for a given hardware thread ID
// In case hardware thread ID is not found -1 is returned
func GetHwthreadDie(cpuID int) int {
	cpuData := CpuData()
	for i := range cpuData {
		d := &cpuData[i]
		if d.CpuID == cpuID {
			return d.Die
		}
	}
	return -1
}

// GetHwthreadCore gets the CPU core ID for a given hardware thread ID
// In case hardware thread ID is not found -1 is returned
func GetHwthreadCore(cpuID int) int {
	cpuData := CpuData()
	for i := range cpuData {
		d := &cpuData[i]
		if d.CpuID == cpuID {
			return d.Core
		}
	}
	return -1
}

// GetSocketHwthreads gets all hardware thread IDs associated with a CPU socket
func GetSocketHwthreads(socket int) []int {
	cpuData := CpuData()
	cpuList := make([]int, 0)
	for i := range cpuData {
		d := &cpuData[i]
		if d.Socket == socket {
			cpuList = append(cpuList, d.CpuID)
		}
	}
	return cpuList
}

// GetNumaDomainHwthreads gets the all hardware thread IDs associated with a NUMA domain
func GetNumaDomainHwthreads(numaDomain int) []int {
	cpuData := CpuData()
	cpuList := make([]int, 0)
	for i := range cpuData {
		d := &cpuData[i]
		if d.NumaDomain == numaDomain {
			cpuList = append(cpuList, d.CpuID)
		}
	}
	return cpuList
}

// GetDieHwthreads gets all hardware thread IDs associated with a CPU die
func GetDieHwthreads(die int) []int {
	cpuData := CpuData()
	cpuList := make([]int, 0)
	for i := range cpuData {
		d := &cpuData[i]
		if d.Die == die {
			cpuList = append(cpuList, d.CpuID)
		}
	}
	return cpuList
}

// GetCoreHwthreads get all hardware thread IDs associated with a CPU core
func GetCoreHwthreads(core int) []int {
	cpuData := CpuData()
	cpuList := make([]int, 0)
	for i := range cpuData {
		d := &cpuData[i]
		if d.Core == core {
			cpuList = append(cpuList, d.CpuID)
		}
	}
	return cpuList
}
