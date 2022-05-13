package collectors

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"strconv"
	"strings"
	"time"

	lp "github.com/ClusterCockpit/cc-metric-collector/internal/ccMetric"
)

type MetricCollector interface {
	Name() string                      // Name of the metric collector
	Init(config json.RawMessage) error // Initialize metric collector
	Initialized() bool                 // Is metric collector initialized?
	Parallel() bool
	Read(duration time.Duration, output chan lp.CCMetric) // Read metrics from metric collector
	Close()                                               // Close / finish metric collector
}

type metricCollector struct {
	name     string            // name of the metric
	init     bool              // is metric collector initialized?
	parallel bool              // can the metric collector be executed in parallel with others
	meta     map[string]string // static meta data tags
}

// Name returns the name of the metric collector
func (c *metricCollector) Name() string {
	return c.name
}

// Name returns the name of the metric collector
func (c *metricCollector) Parallel() bool {
	return c.parallel
}

// Setup is for future use
func (c *metricCollector) setup() error {
	return nil
}

// Initialized indicates whether the metric collector has been initialized
func (c *metricCollector) Initialized() bool {
	return c.init
}

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
func stringArrayContains(array []string, str string) (int, bool) {
	for i, a := range array {
		if a == str {
			return i, true
		}
	}
	return -1, false
}

// SocketList returns the list of physical sockets as read from /proc/cpuinfo
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

// CpuList returns the list of physical CPUs (in contrast to logical CPUs) as read from /proc/cpuinfo
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

// RemoveFromStringList removes the string r from the array of strings s
// If r is not contained in the array an error is returned
func RemoveFromStringList(s []string, r string) ([]string, error) {
	for i := range s {
		if r == s[i] {
			return append(s[:i], s[i+1:]...), nil
		}
	}
	return s, fmt.Errorf("no such string in list")
}
