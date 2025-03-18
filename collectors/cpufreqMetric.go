package collectors

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	cclog "github.com/ClusterCockpit/cc-lib/ccLogger"
	lp "github.com/ClusterCockpit/cc-lib/ccMessage"
	"github.com/ClusterCockpit/cc-metric-collector/pkg/ccTopology"
	"golang.org/x/sys/unix"
)

type CPUFreqCollectorTopology struct {
	scalingCurFreqFile string
	tagSet             map[string]string
}

// CPUFreqCollector
// a metric collector to measure the current frequency of the CPUs
// as obtained from the hardware (in KHz)
// Only measure on the first hyper-thread
//
// See: https://www.kernel.org/doc/html/latest/admin-guide/pm/cpufreq.html
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
	m.parallel = true
	if len(config) > 0 {
		err := json.Unmarshal(config, &m.config)
		if err != nil {
			return err
		}
	}
	m.meta = map[string]string{
		"source": m.name,
		"group":  "CPU",
		"unit":   "Hz",
	}

	m.topology = make([]CPUFreqCollectorTopology, 0)
	for _, c := range ccTopology.CpuData() {

		// Skip hyper threading CPUs
		if c.CpuID != c.CoreCPUsList[0] {
			continue
		}

		// Check access to current frequency file
		scalingCurFreqFile := filepath.Join("/sys/devices/system/cpu", fmt.Sprintf("cpu%d", c.CpuID), "cpufreq/scaling_cur_freq")
		err := unix.Access(scalingCurFreqFile, unix.R_OK)
		if err != nil {
			return fmt.Errorf("unable to access file '%s': %v", scalingCurFreqFile, err)
		}

		m.topology = append(m.topology,
			CPUFreqCollectorTopology{
				tagSet: map[string]string{
					"type":       "hwthread",
					"type-id":    fmt.Sprint(c.CpuID),
					"package_id": fmt.Sprint(c.Socket),
				},
				scalingCurFreqFile: scalingCurFreqFile,
			},
		)
	}

	// Initialized
	cclog.ComponentDebug(
		m.name,
		"initialized",
		len(m.topology), "non-hyper-threading CPUs")
	m.init = true
	return nil
}

func (m *CPUFreqCollector) Read(interval time.Duration, output chan lp.CCMessage) {
	// Check if already initialized
	if !m.init {
		return
	}

	now := time.Now()
	for i := range m.topology {
		t := &m.topology[i]

		// Read current frequency
		line, err := os.ReadFile(t.scalingCurFreqFile)
		if err != nil {
			cclog.ComponentError(
				m.name,
				fmt.Sprintf("Read(): Failed to read file '%s': %v", t.scalingCurFreqFile, err))
			continue
		}
		cpuFreq, err := strconv.ParseInt(strings.TrimSpace(string(line)), 10, 64)
		if err != nil {
			cclog.ComponentError(
				m.name,
				fmt.Sprintf("Read(): Failed to convert CPU frequency '%s' to int64: %v", line, err))
			continue
		}

		if y, err := lp.NewMessage("cpufreq", t.tagSet, m.meta, map[string]interface{}{"value": cpuFreq}, now); err == nil {
			output <- y
		}
	}
}

func (m *CPUFreqCollector) Close() {
	m.init = false
}
