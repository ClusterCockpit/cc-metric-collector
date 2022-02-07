package collectors

import (
	"bufio"
	"encoding/json"

	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	cclog "github.com/ClusterCockpit/cc-metric-collector/internal/ccLogger"
	lp "github.com/ClusterCockpit/cc-metric-collector/internal/ccMetric"
)

//
// CPUFreqCollector
// a metric collector to measure the current frequency of the CPUs
// as obtained from /proc/cpuinfo
// Only measure on the first hyperthread
//
type CPUFreqCpuInfoCollectorTopology struct {
	processor               string // logical processor number (continuous, starting at 0)
	coreID                  string // socket local core ID
	coreID_int              int64
	physicalPackageID       string // socket / package ID
	physicalPackageID_int   int64
	numPhysicalPackages     string // number of  sockets / packages
	numPhysicalPackages_int int64
	isHT                    bool
	numNonHT                string // number of non hyperthreading processors
	numNonHT_int            int64
	tagSet                  map[string]string
}

type CPUFreqCpuInfoCollector struct {
	metricCollector
	topology []CPUFreqCpuInfoCollectorTopology
}

func (m *CPUFreqCpuInfoCollector) Init(config json.RawMessage) error {
	// Check if already initialized
	if m.init {
		return nil
	}

	m.name = "CPUFreqCpuInfoCollector"
	m.meta = map[string]string{
		"source": m.name,
		"group":  "CPU",
		"unit":   "MHz",
	}

	const cpuInfoFile = "/proc/cpuinfo"
	file, err := os.Open(cpuInfoFile)
	if err != nil {
		return fmt.Errorf("Failed to open file '%s': %v", cpuInfoFile, err)
	}
	defer file.Close()

	// Collect topology information from file cpuinfo
	foundFreq := false
	processor := ""
	var numNonHT_int int64 = 0
	coreID := ""
	physicalPackageID := ""
	var maxPhysicalPackageID int64 = 0
	m.topology = make([]CPUFreqCpuInfoCollectorTopology, 0)
	coreSeenBefore := make(map[string]bool)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lineSplit := strings.Split(scanner.Text(), ":")
		if len(lineSplit) == 2 {
			key := strings.TrimSpace(lineSplit[0])
			value := strings.TrimSpace(lineSplit[1])
			switch key {
			case "cpu MHz":
				// frequency
				foundFreq = true
			case "processor":
				processor = value
			case "core id":
				coreID = value
			case "physical id":
				physicalPackageID = value
			}
		}

		// were all topology information collected?
		if foundFreq &&
			len(processor) > 0 &&
			len(coreID) > 0 &&
			len(physicalPackageID) > 0 {

			coreID_int, err := strconv.ParseInt(coreID, 10, 64)
			if err != nil {
				return fmt.Errorf("Unable to convert coreID '%s' to int64: %v", coreID, err)
			}
			physicalPackageID_int, err := strconv.ParseInt(physicalPackageID, 10, 64)
			if err != nil {
				return fmt.Errorf("Unable to convert physicalPackageID '%s' to int64: %v", physicalPackageID, err)
			}

			// increase maximun socket / package ID, when required
			if physicalPackageID_int > maxPhysicalPackageID {
				maxPhysicalPackageID = physicalPackageID_int
			}

			globalID := physicalPackageID + ":" + coreID
			isHT := coreSeenBefore[globalID]
			coreSeenBefore[globalID] = true
			if !isHT {
				// increase number on non hyper thread cores
				numNonHT_int++
			}

			// store collected topology information
			m.topology = append(
				m.topology,
				CPUFreqCpuInfoCollectorTopology{
					processor:             processor,
					coreID:                coreID,
					coreID_int:            coreID_int,
					physicalPackageID:     physicalPackageID,
					physicalPackageID_int: physicalPackageID_int,
					isHT:                  isHT,
				})

			// reset topology information
			foundFreq = false
			processor = ""
			coreID = ""
			physicalPackageID = ""
		}
	}

	numPhysicalPackageID_int := maxPhysicalPackageID + 1
	numPhysicalPackageID := fmt.Sprint(numPhysicalPackageID_int)
	numNonHT := fmt.Sprint(numNonHT_int)
	for i := range m.topology {
		t := &m.topology[i]
		t.numPhysicalPackages = numPhysicalPackageID
		t.numPhysicalPackages_int = numPhysicalPackageID_int
		t.numNonHT = numNonHT
		t.numNonHT_int = numNonHT_int
		t.tagSet = map[string]string{
			"type":        "cpu",
			"type-id":     t.processor,
			"num_core":    t.numNonHT,
			"package_id":  t.physicalPackageID,
			"num_package": t.numPhysicalPackages,
		}
	}

	m.init = true
	return nil
}

func (m *CPUFreqCpuInfoCollector) Read(interval time.Duration, output chan lp.CCMetric) {
	// Check if already initialized
	if !m.init {
		return
	}

	const cpuInfoFile = "/proc/cpuinfo"
	file, err := os.Open(cpuInfoFile)
	if err != nil {
		cclog.ComponentError(
			m.name,
			fmt.Sprintf("Read(): Failed to open file '%s': %v", cpuInfoFile, err))
		return
	}
	defer file.Close()

	processorCounter := 0
	now := time.Now()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lineSplit := strings.Split(scanner.Text(), ":")
		if len(lineSplit) == 2 {
			key := strings.TrimSpace(lineSplit[0])

			// frequency
			if key == "cpu MHz" {
				t := &m.topology[processorCounter]
				if !t.isHT {
					value, err := strconv.ParseFloat(strings.TrimSpace(lineSplit[1]), 64)
					if err != nil {
						cclog.ComponentError(
							m.name,
							fmt.Sprintf("Read(): Failed to convert cpu MHz '%s' to float64: %v", lineSplit[1], err))
						return
					}
					if y, err := lp.New("cpufreq", t.tagSet, m.meta, map[string]interface{}{"value": value}, now); err == nil {
						output <- y
					}
				}
				processorCounter++
			}
		}
	}
}

func (m *CPUFreqCpuInfoCollector) Close() {
	m.init = false
}
