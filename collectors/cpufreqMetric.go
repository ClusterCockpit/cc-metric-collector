package collectors

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	lp "github.com/ClusterCockpit/cc-metric-collector/internal/ccMetric"
	"golang.org/x/sys/unix"
)

//
// readOneLine reads one line from a file.
// It returns ok when file was successfully read.
// In this case text contains the first line of the files contents.
//
func readOneLine(filename string) (text string, ok bool) {
	file, err := os.Open(filename)
	if err != nil {
		return
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	ok = scanner.Scan()
	text = scanner.Text()
	return
}

type CPUFreqCollectorCPU struct {
	// coreID, packageID, num_cores, num_package
	tagSet             map[string]string
	scalingCurFreqFile string
}

//
// CPUFreqCollector
// a metric collector to measure the current frequency of the CPUs
// as obtained from the hardware (in KHz)
// Only measure on the first hyper thread
//
// See: https://www.kernel.org/doc/html/latest/admin-guide/pm/cpufreq.html
//
type CPUFreqCollector struct {
	metricCollector
	config struct {
		ExcludeMetrics []string `json:"exclude_metrics,omitempty"`
	}
	cpus []CPUFreqCollectorCPU
}

func (m *CPUFreqCollector) Init(config json.RawMessage) error {
	m.name = "CPUFreqCollector"
	m.setup()
	if len(config) > 0 {
		err := json.Unmarshal(config, &m.config)
		if err != nil {
			return err
		}
	}
	m.meta = map[string]string{
		"source": m.name,
		"group":  "CPU Frequency",
	}

	// Initialize CPU list
	m.cpus = make([]CPUFreqCollectorCPU, 0)

	// Loop for all CPU directories
	baseDir := "/sys/devices/system/cpu"
	globPattern := filepath.Join(baseDir, "cpu[0-9]*")
	cpuDirs, err := filepath.Glob(globPattern)
	if err != nil {
		return fmt.Errorf("CPUFreqCollector.Init() unable to glob files with pattern %s: %v", globPattern, err)
	}
	if cpuDirs == nil {
		return fmt.Errorf("CPUFreqCollector.Init() unable to find any files with pattern %s", globPattern)
	}

	maxPackageID := 0
	maxCoreID := 0
	for _, cpuDir := range cpuDirs {
		cpuID := strings.TrimPrefix(cpuDir, "/sys/devices/system/cpu/cpu")

		// Read thread sibling list
		threadSiblingListFile := filepath.Join(cpuDir, "topology", "thread_siblings_list")
		threadSiblingList, ok := readOneLine(threadSiblingListFile)
		if !ok {
			return fmt.Errorf("CPUFreqCollector.Init() unable to read thread siblings list from %s", threadSiblingListFile)
		}

		// Read frequency only from first hardware thread
		// Ignore Simultaneous Multithreading (SMT) / Hyper-Threading
		if strings.Split(threadSiblingList, ",")[0] == cpuID {
			// Read package ID
			packageIDFile := filepath.Join(cpuDir, "topology", "physical_package_id")
			packageID, ok := readOneLine(packageIDFile)
			if !ok {
				return fmt.Errorf("CPUFreqCollector.Init() unable to read physical package ID from %s", packageIDFile)
			}
			packageID_int, err := strconv.Atoi(packageID)
			if err != nil {
				return fmt.Errorf("CPUFreqCollector.Init() unable to convert packageID to int: %v", err)
			}

			// Update maxPackageID
			if packageID_int > maxPackageID {
				maxPackageID = packageID_int
			}

			// Read core ID
			coreIDFile := filepath.Join(cpuDir, "topology", "core_id")
			coreID, ok := readOneLine(coreIDFile)
			if !ok {
				return fmt.Errorf("CPUFreqCollector.Init() unable to read core ID from %s", coreIDFile)
			}
			coreID_int, err := strconv.Atoi(coreID)
			if err != nil {
				return fmt.Errorf("CPUFreqCollector.Init() unable to convert coreID to int: %v", err)
			}

			// Update maxCoreID
			if coreID_int > maxCoreID {
				maxCoreID = coreID_int
			}

			// Check access to current frequency file
			scalingCurFreqFile := filepath.Join(cpuDir, "cpufreq", "scaling_cur_freq")
			err = unix.Access(scalingCurFreqFile, unix.R_OK)
			if err != nil {
				return fmt.Errorf("CPUFreqCollector.Init() unable to access %s: %v", scalingCurFreqFile, err)
			}

			m.cpus = append(
				m.cpus,
				CPUFreqCollectorCPU{
					tagSet: map[string]string{
						"type":      "cpu",
						"type-id":   strings.TrimSpace(coreID),
						"packageID": strings.TrimSpace(packageID),
					},
					scalingCurFreqFile: scalingCurFreqFile,
				})
		}
	}

	// Add num packages and num cores as tags
	numPackages := strconv.Itoa(maxPackageID + 1)
	numCores := strconv.Itoa(maxCoreID + 1)
	for i := range m.cpus {
		c := &m.cpus[i]
		c.tagSet["num_core"] = numCores
		c.tagSet["num_package"] = numPackages
	}

	m.init = true
	return nil
}

func (m *CPUFreqCollector) Read(interval time.Duration, output chan lp.CCMetric) {
	if !m.init {
		return
	}

	now := time.Now()
	for i := range m.cpus {
		cpu := &m.cpus[i]

		// Read current frequency
		line, ok := readOneLine(cpu.scalingCurFreqFile)
		if !ok {
			log.Printf("CPUFreqCollector.Read(): Failed to read one line from file '%s'", cpu.scalingCurFreqFile)
			continue
		}
		cpuFreq, err := strconv.Atoi(line)
		if err != nil {
			log.Printf("CPUFreqCollector.Read(): Failed to convert CPU frequency '%s': %v", line, err)
			continue
		}

		y, err := lp.New("cpufreq", cpu.tagSet, m.meta, map[string]interface{}{"value": cpuFreq}, now)
		if err == nil {
			output <- y
		}
	}
}

func (m *CPUFreqCollector) Close() {
	m.init = false
}
