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

	lp "github.com/ClusterCockpit/cc-lib/ccMessage"
	cclog "github.com/ClusterCockpit/cc-metric-collector/pkg/ccLogger"
)

// Non-Uniform Memory Access (NUMA) policy hit/miss statistics
//
// numa_hit:
//
//	A process wanted to allocate memory from this node, and succeeded.
//
// numa_miss:
//
//	A process wanted to allocate memory from another node,
//	but ended up with memory from this node.
//
// numa_foreign:
//
//	A process wanted to allocate on this node,
//	but ended up with memory from another node.
//
// local_node:
//
//	A process ran on this node's CPU,
//	and got memory from this node.
//
// other_node:
//
//	A process ran on a different node's CPU
//	and got memory from this node.
//
// interleave_hit:
//
//	Interleaving wanted to allocate from this node
//	and succeeded.
//
// See: https://www.kernel.org/doc/html/latest/admin-guide/numastat.html
type NUMAStatsCollectorConfig struct {
	SendAbsoluteValues bool     `json:"send_abs_values"`     // Defaults to true if not provided.
	SendDiffValues     bool     `json:"send_diff_values"`    // If true, diff metrics are sent.
	SendDerivedValues  bool     `json:"send_derived_values"` // If true, derived (rate) metrics are sent.
	ExcludeMetrics     []string `json:"exclude_metrics,omitempty"`
	OnlyMetrics        []string `json:"only_metrics,omitempty"`
}

// NUMAStatsCollectorTopology represents a single NUMA domain.
type NUMAStatsCollectorTopolgy struct {
	file           string
	tagSet         map[string]string
	previousValues map[string]int64
}

// NUMAStatsCollector collects NUMA statistics from /sys/devices/system/node/node*/numastat.
type NUMAStatsCollector struct {
	metricCollector
	topology      []NUMAStatsCollectorTopolgy
	config        NUMAStatsCollectorConfig
	lastTimestamp time.Time
}

type NUMAMetricDefinition struct {
	name string
	unit string
}

// shouldOutput returns true if a metric should be forwarded based on only_metrics and exclude_metrics.
func (m *NUMAStatsCollector) shouldOutput(metricName string) bool {
	if len(m.config.OnlyMetrics) > 0 {
		for _, n := range m.config.OnlyMetrics {
			if n == metricName {
				return true
			}
		}
		return false
	}
	for _, n := range m.config.ExcludeMetrics {
		if n == metricName {
			return false
		}
	}
	return true
}

func (m *NUMAStatsCollector) Init(config json.RawMessage) error {
	if m.init {
		return nil
	}

	m.name = "NUMAStatsCollector"
	m.parallel = true
	m.setup()
	m.meta = map[string]string{
		"source": m.name,
		"group":  "NUMA",
	}
	// Default configuration: send_abs_values defaults to true.
	m.config.SendAbsoluteValues = true
	if len(config) > 0 {
		if err := json.Unmarshal(config, &m.config); err != nil {
			return err
		}
	}
	base := "/sys/devices/system/node/node"
	globPattern := base + "[0-9]*"
	dirs, err := filepath.Glob(globPattern)
	if err != nil {
		return fmt.Errorf("unable to glob files with pattern '%s'", globPattern)
	}
	if dirs == nil {
		return fmt.Errorf("unable to find any files with pattern '%s'", globPattern)
	}
	m.topology = make([]NUMAStatsCollectorTopolgy, 0, len(dirs))
	for _, dir := range dirs {
		node := strings.TrimPrefix(dir, base)
		file := filepath.Join(dir, "numastat")
		m.topology = append(m.topology, NUMAStatsCollectorTopolgy{
			file:           file,
			tagSet:         map[string]string{"memoryDomain": node},
			previousValues: make(map[string]int64),
		})
	}
	cclog.ComponentDebug(m.name, "initialized", len(m.topology), "NUMA domains")
	m.lastTimestamp = time.Now()
	m.init = true
	return nil
}

func (m *NUMAStatsCollector) Read(interval time.Duration, output chan lp.CCMessage) {
	if !m.init {
		return
	}

	now := time.Now()
	timeDiff := now.Sub(m.lastTimestamp).Seconds()
	m.lastTimestamp = now

	for i := range m.topology {
		t := &m.topology[i]
		file, err := os.Open(t.file)
		if err != nil {
			cclog.ComponentError(m.name, fmt.Sprintf("Read(): Failed to open file '%s': %v", t.file, err))
			continue
		}
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Text()
			split := strings.Fields(line)
			if len(split) != 2 {
				continue
			}
			key := split[0]
			value, err := strconv.ParseInt(split[1], 10, 64)
			if err != nil {
				cclog.ComponentError(m.name, fmt.Sprintf("Read(): Failed to convert %s='%s' to int64: %v", key, split[1], err))
				continue
			}
			baseName := "numastats_" + key

			// Send absolute value if enabled.
			if m.config.SendAbsoluteValues && m.shouldOutput(baseName) {
				msg, err := lp.NewMessage(baseName, t.tagSet, m.meta, map[string]interface{}{"value": value}, now)
				if err == nil {
					msg.AddMeta("unit", "count")
					output <- msg
				}
			}

			// If a previous value exists, compute diff and derived.
			if prev, ok := t.previousValues[key]; ok {
				diff := value - prev
				if m.config.SendDiffValues && m.shouldOutput(baseName+"_diff") {
					msg, err := lp.NewMessage(baseName+"_diff", t.tagSet, m.meta, map[string]interface{}{"value": diff}, now)
					if err == nil {
						msg.AddMeta("unit", "count")
						output <- msg
					}
				}
				if m.config.SendDerivedValues && m.shouldOutput(baseName+"_rate") {
					rate := float64(value-prev) / timeDiff
					msg, err := lp.NewMessage(baseName+"_rate", t.tagSet, m.meta, map[string]interface{}{"value": rate}, now)
					if err == nil {
						msg.AddMeta("unit", "counts/s")
						output <- msg
					}
				}
			}
			t.previousValues[key] = value
		}
		file.Close()
	}
}

func (m *NUMAStatsCollector) Close() {
	m.init = false
}
