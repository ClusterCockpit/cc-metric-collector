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

	lp "github.com/influxdata/line-protocol"
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
	coreID_int              int
	physicalPackageID       string // socket / package ID
	physicalPackageID_int   int
	numPhysicalPackages     string // number of  sockets / packages
	numPhysicalPackages_int int
	isHT                    bool
	numNonHT                string // number of non hyperthreading processors
	numNonHT_int            int
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
	MetricCollector
	topology []CPUFreqCollectorTopology
	config   struct {
		ExcludeMetrics []string `json:"exclude_metrics,omitempty"`
	}
}

func (m *CPUFreqCollector) Init(config []byte) error {
	m.name = "CPUFreqCollector"
	m.setup()
	if len(config) > 0 {
		err := json.Unmarshal(config, &m.config)
		if err != nil {
			return err
		}
	}

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

	// Initialize CPU topology
	m.topology = make([]CPUFreqCollectorTopology, len(cpuDirs))
	for _, cpuDir := range cpuDirs {
		processor := strings.TrimPrefix(cpuDir, "/sys/devices/system/cpu/cpu")
		processor_int, err := strconv.Atoi(processor)
		if err != nil {
			return fmt.Errorf("CPUFreqCollector.Init() unable to convert cpuID to int: %v", err)
		}

		// Read package ID
		physicalPackageIDFile := filepath.Join(cpuDir, "topology", "physical_package_id")
		physicalPackageID, ok := readOneLine(physicalPackageIDFile)
		if !ok {
			return fmt.Errorf("CPUFreqCollector.Init() unable to read physical package ID from %s", physicalPackageIDFile)
		}
		physicalPackageID_int, err := strconv.Atoi(physicalPackageID)
		if err != nil {
			return fmt.Errorf("CPUFreqCollector.Init() unable to convert packageID to int: %v", err)
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

		// Check access to current frequency file
		scalingCurFreqFile := filepath.Join(cpuDir, "cpufreq", "scaling_cur_freq")
		err = unix.Access(scalingCurFreqFile, unix.R_OK)
		if err != nil {
			return fmt.Errorf("CPUFreqCollector.Init() unable to access %s: %v", scalingCurFreqFile, err)
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
	numNonHT_int := 0
	maxPhysicalPackageID := 0
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

func (m *CPUFreqCollector) Read(interval time.Duration, out *[]lp.MutableMetric) {
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
			log.Printf("CPUFreqCollector.Read(): Failed to read one line from file '%s'", t.scalingCurFreqFile)
			continue
		}
		cpuFreq, err := strconv.Atoi(line)
		if err != nil {
			log.Printf("CPUFreqCollector.Read(): Failed to convert CPU frequency '%s': %v", line, err)
			continue
		}

		y, err := lp.New("cpufreq", t.tagSet, map[string]interface{}{"value": cpuFreq}, now)
		if err == nil {
			*out = append(*out, y)
		}
	}
}

func (m *CPUFreqCollector) Close() {
	m.init = false
}
