package collectors

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	cclog "github.com/ClusterCockpit/cc-metric-collector/internal/ccLogger"
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

type CPUFreqCollectorTopology struct {
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
	scalingCurFreqFile      string
	tagSet                  map[string]string
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
	topology []CPUFreqCollectorTopology
	config   struct {
		ExcludeMetrics []string `json:"exclude_metrics,omitempty"`
	}
}

func (m *CPUFreqCollector) Init(config json.RawMessage) error {
	// Check if already initialized
	if m.init {
		return nil
	}

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
		"group":  "CPU",
		"unit":   "MHz",
	}

	// Loop for all CPU directories
	baseDir := "/sys/devices/system/cpu"
	globPattern := filepath.Join(baseDir, "cpu[0-9]*")
	cpuDirs, err := filepath.Glob(globPattern)
	if err != nil {
		return fmt.Errorf("Unable to glob files with pattern '%s': %v", globPattern, err)
	}
	if cpuDirs == nil {
		return fmt.Errorf("Unable to find any files with pattern '%s'", globPattern)
	}

	// Initialize CPU topology
	m.topology = make([]CPUFreqCollectorTopology, len(cpuDirs))
	for _, cpuDir := range cpuDirs {
		processor := strings.TrimPrefix(cpuDir, "/sys/devices/system/cpu/cpu")
		processor_int, err := strconv.ParseInt(processor, 10, 64)
		if err != nil {
			return fmt.Errorf("Unable to convert cpuID '%s' to int64: %v", processor, err)
		}

		// Read package ID
		physicalPackageIDFile := filepath.Join(cpuDir, "topology", "physical_package_id")
		physicalPackageID, ok := readOneLine(physicalPackageIDFile)
		if !ok {
			return fmt.Errorf("Unable to read physical package ID from file '%s'", physicalPackageIDFile)
		}
		physicalPackageID_int, err := strconv.ParseInt(physicalPackageID, 10, 64)
		if err != nil {
			return fmt.Errorf("Unable to convert packageID '%s' to int64: %v", physicalPackageID, err)
		}

		// Read core ID
		coreIDFile := filepath.Join(cpuDir, "topology", "core_id")
		coreID, ok := readOneLine(coreIDFile)
		if !ok {
			return fmt.Errorf("Unable to read core ID from file '%s'", coreIDFile)
		}
		coreID_int, err := strconv.ParseInt(coreID, 10, 64)
		if err != nil {
			return fmt.Errorf("Unable to convert coreID '%s' to int64: %v", coreID, err)
		}

		// Check access to current frequency file
		scalingCurFreqFile := filepath.Join(cpuDir, "cpufreq", "scaling_cur_freq")
		err = unix.Access(scalingCurFreqFile, unix.R_OK)
		if err != nil {
			return fmt.Errorf("Unable to access file '%s': %v", scalingCurFreqFile, err)
		}

		t := &m.topology[processor_int]
		t.processor = processor
		t.physicalPackageID = physicalPackageID
		t.physicalPackageID_int = physicalPackageID_int
		t.coreID = coreID
		t.coreID_int = coreID_int
		t.scalingCurFreqFile = scalingCurFreqFile
	}

	// is processor a hyperthread?
	coreSeenBefore := make(map[string]bool)
	for i := range m.topology {
		t := &m.topology[i]

		globalID := t.physicalPackageID + ":" + t.coreID
		t.isHT = coreSeenBefore[globalID]
		coreSeenBefore[globalID] = true
	}

	// number of non hyper thread cores and packages / sockets
	var numNonHT_int int64 = 0
	var maxPhysicalPackageID int64 = 0
	for i := range m.topology {
		t := &m.topology[i]

		// Update maxPackageID
		if t.physicalPackageID_int > maxPhysicalPackageID {
			maxPhysicalPackageID = t.physicalPackageID_int
		}

		if !t.isHT {
			numNonHT_int++
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

func (m *CPUFreqCollector) Read(interval time.Duration, output chan lp.CCMetric) {
	// Check if already initialized
	if !m.init {
		return
	}

	now := time.Now()
	for i := range m.topology {
		t := &m.topology[i]

		// skip hyperthreads
		if t.isHT {
			continue
		}

		// Read current frequency
		line, ok := readOneLine(t.scalingCurFreqFile)
		if !ok {
			cclog.ComponentError(
				m.name,
				fmt.Sprintf("Read(): Failed to read one line from file '%s'", t.scalingCurFreqFile))
			continue
		}
		cpuFreq, err := strconv.ParseInt(line, 10, 64)
		if err != nil {
			cclog.ComponentError(
				m.name,
				fmt.Sprintf("Read(): Failed to convert CPU frequency '%s' to int64: %v", line, err))
			continue
		}

		if y, err := lp.New("cpufreq", t.tagSet, m.meta, map[string]interface{}{"value": cpuFreq}, now); err == nil {
			output <- y
		}
	}
}

func (m *CPUFreqCollector) Close() {
	m.init = false
}
