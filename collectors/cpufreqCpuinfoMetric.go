package collectors

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	lp "github.com/ClusterCockpit/cc-metric-collector/internal/ccMetric"
)

//
// CPUFreqCollector
// a metric collector to measure the current frequency of the CPUs
// as obtained from /proc/cpuinfo
// Only measure on the first hyperthread
//
type CPUFreqCpuInfoCollectorTopology struct {
	processor     string // logical processor number (continuous, starting at 0)
	coreID        string // socket local core ID
	physicalID    string // socket / package ID
	numPhysicalID string // number of  sockets / packages
	isHT          bool
	numNonHT      string // number of non hyperthreading processors
	tagSet        map[string]string
}

type CPUFreqCpuInfoCollector struct {
	metricCollector
	topology []CPUFreqCpuInfoCollectorTopology
}

func (m *CPUFreqCpuInfoCollector) Init(config json.RawMessage) error {
	m.name = "CPUFreqCpuInfoCollector"
	m.meta = map[string]string{
		"source": m.name,
		"group":  "cpufreq",
	}

	const cpuInfoFile = "/proc/cpuinfo"
	file, err := os.Open(cpuInfoFile)
	if err != nil {
		return fmt.Errorf("Failed to open '%s': %v", cpuInfoFile, err)
	}
	defer file.Close()

	// Collect topology information from file cpuinfo
	foundFreq := false
	processor := ""
	numNonHT := 0
	coreID := ""
	physicalID := ""
	maxPhysicalID := 0
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
				physicalID = value
			}
		}

		// were all topology information collected?
		if foundFreq &&
			len(processor) > 0 &&
			len(coreID) > 0 &&
			len(physicalID) > 0 {

			globalID := physicalID + ":" + coreID
			isHT := coreSeenBefore[globalID]
			coreSeenBefore[globalID] = true
			if !isHT {
				// increase number on non hyper thread cores
				numNonHT++

				// increase maximun socket / package ID, when required
				physicalIDInt, err := strconv.Atoi(physicalID)
				if err != nil {
					return fmt.Errorf("Failed to convert physical id to int: %v", err)
				}
				if physicalIDInt > maxPhysicalID {
					maxPhysicalID = physicalIDInt
				}
			}

			// store collected topology information
			m.topology = append(
				m.topology,
				CPUFreqCpuInfoCollectorTopology{
					processor:  processor,
					coreID:     coreID,
					physicalID: physicalID,
					isHT:       isHT,
				})

			// reset topology information
			foundFreq = false
			processor = ""
			coreID = ""
			physicalID = ""
		}
	}

	numPhysicalID := fmt.Sprint(maxPhysicalID + 1)
	numNonHTString := fmt.Sprint(numNonHT)
	for i := range m.topology {
		t := &m.topology[i]
		t.numPhysicalID = numPhysicalID
		t.numNonHT = numNonHTString
		t.tagSet = map[string]string{
			"type":        "cpu",
			"type-id":     t.processor,
			"num_core":    t.numNonHT,
			"package_id":  t.physicalID,
			"num_package": t.numPhysicalID,
		}
	}

	m.init = true
	return nil
}

func (m *CPUFreqCpuInfoCollector) Read(interval time.Duration, output chan lp.CCMetric) {
	if !m.init {
		return
	}
	const cpuInfoFile = "/proc/cpuinfo"
	file, err := os.Open(cpuInfoFile)
	if err != nil {
		log.Printf("Failed to open '%s': %v", cpuInfoFile, err)
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
						log.Printf("Failed to convert cpu MHz to float: %v", err)
						return
					}
					y, err := lp.New("cpufreq", t.tagSet, m.meta, map[string]interface{}{"value": value}, now)
					if err == nil {
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
