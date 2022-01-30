package ccTopology

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	cclogger "github.com/ClusterCockpit/cc-metric-collector/internal/ccLogger"
)

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

// stringArrayContains scans an array of strings if the value str is present in the array
// If the specified value is found, the corresponding array index is returned.
// The bool value is used to signal success or failure
// func stringArrayContains(array []string, str string) (int, bool) {
// 	for i, a := range array {
// 		if a == str {
// 			return i, true
// 		}
// 	}
// 	return -1, false
// }

func SocketList() []int {
	buffer, err := ioutil.ReadFile("/proc/cpuinfo")
	if err != nil {
		log.Print(err)
		return nil
	}
	ll := strings.Split(string(buffer), "\n")
	var packs []int
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

func CpuList() []int {
	buffer, err := ioutil.ReadFile("/proc/cpuinfo")
	if err != nil {
		log.Print(err)
		return nil
	}
	ll := strings.Split(string(buffer), "\n")
	var cpulist []int
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

type CpuEntry struct {
	Cpuid      int
	SMT        int
	Core       int
	Socket     int
	Numadomain int
	Die        int
}

func CpuData() []CpuEntry {

	fileToInt := func(path string) int {
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
			log.Print(err)
		}
		threadlist := make([]int, 0)
		sbuffer := strings.Replace(string(buffer), "\n", "", -1)
		for _, x := range strings.Split(sbuffer, ",") {
			id, err := strconv.ParseInt(x, 10, 32)
			if err != nil {
				log.Print(err)
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
		files, err := filepath.Glob(fmt.Sprintf("%s/node*", basepath))
		if err != nil {
			log.Print(err)
		}
		for _, f := range files {
			finfo, err := os.Lstat(f)
			if err == nil && (finfo.IsDir() || finfo.Mode()&os.ModeSymlink != 0) {
				var id int
				parts := strings.Split(f, "/")
				_, err = fmt.Scanf("node%d", parts[len(parts)-1], &id)
				if err == nil {
					return id
				}
			}
		}
		return 0
	}

	clist := make([]CpuEntry, 0)
	for _, c := range CpuList() {
		clist = append(clist, CpuEntry{Cpuid: c})
	}
	for _, centry := range clist {
		centry.Socket = -1
		centry.Numadomain = -1
		centry.Die = -1
		centry.Core = -1
		// Set base directory for topology lookup
		base := fmt.Sprintf("/sys/devices/system/cpu/cpu%d/topology", centry.Cpuid)

		// Lookup CPU core id
		centry.Core = getCore(base)

		// Lookup CPU socket id
		centry.Socket = getSocket(base)

		// Lookup CPU die id
		centry.Die = getDie(base)

		// Lookup SMT thread id
		centry.SMT = getSMT(centry.Cpuid, base)

		// Lookup NUMA domain id
		centry.Numadomain = getNumaDomain(base)

	}
	return clist
}

type CpuInformation struct {
	NumHWthreads   int
	SMTWidth       int
	NumSockets     int
	NumDies        int
	NumNumaDomains int
}

func CpuInfo() CpuInformation {
	var c CpuInformation

	smt := 0
	numa := 0
	die := 0
	socket := 0
	cdata := CpuData()
	for _, d := range cdata {
		if d.SMT > smt {
			smt = d.SMT
		}
		if d.Numadomain > numa {
			numa = d.Numadomain
		}
		if d.Die > die {
			die = d.Die
		}
		if d.Socket > socket {
			socket = d.Socket
		}
	}
	c.NumNumaDomains = numa + 1
	c.SMTWidth = smt + 1
	c.NumDies = die + 1
	c.NumSockets = socket + 1
	c.NumHWthreads = len(cdata)
	return c
}

func GetCpuSocket(cpuid int) int {
	cdata := CpuData()
	for _, d := range cdata {
		if d.Cpuid == cpuid {
			return d.Socket
		}
	}
	return -1
}

func GetCpuNumaDomain(cpuid int) int {
	cdata := CpuData()
	for _, d := range cdata {
		if d.Cpuid == cpuid {
			return d.Numadomain
		}
	}
	return -1
}

func GetCpuDie(cpuid int) int {
	cdata := CpuData()
	for _, d := range cdata {
		if d.Cpuid == cpuid {
			return d.Die
		}
	}
	return -1
}

func GetCpuCore(cpuid int) int {
	cdata := CpuData()
	for _, d := range cdata {
		if d.Cpuid == cpuid {
			return d.Core
		}
	}
	return -1
}
