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

	cclog "github.com/ClusterCockpit/cc-lib/v2/ccLogger"
	lp "github.com/ClusterCockpit/cc-lib/v2/ccMessage"
)

type NUMAStatsCollectorConfig struct {
	SendAbsoluteValues bool `json:"send_abs_values"`
	SendDerivedValues  bool `json:"send_derived_values"`
}

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
type NUMAStatsCollectorTopolgy struct {
	file           string
	tagSet         map[string]string
	previousValues map[string]int64
}

type NUMAStatsCollector struct {
	metricCollector
	topology      []NUMAStatsCollectorTopolgy
	config        NUMAStatsCollectorConfig
	lastTimestamp time.Time
}

func (m *NUMAStatsCollector) Init(config json.RawMessage) error {
	// Check if already initialized
	if m.init {
		return nil
	}

	m.name = "NUMAStatsCollector"
	m.parallel = true
	if err := m.setup(); err != nil {
		return fmt.Errorf("%s Init(): setup() call failed: %w", m.name, err)
	}
	m.meta = map[string]string{
		"source": m.name,
		"group":  "NUMA",
	}

	m.config.SendAbsoluteValues = true
	if len(config) > 0 {
		err := json.Unmarshal(config, &m.config)
		if err != nil {
			return fmt.Errorf("unable to unmarshal numastat configuration: %s", err.Error())
		}
	}

	// Loop for all NUMA node directories
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
		m.topology = append(m.topology,
			NUMAStatsCollectorTopolgy{
				file: file,
				tagSet: map[string]string{
					"type":    "memoryDomain",
					"type-id": node,
				},
				previousValues: make(map[string]int64),
			})
	}

	// Initialized
	cclog.ComponentDebug(m.name, "initialized", len(m.topology), "NUMA domains")
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
		// Loop for all NUMA domains
		t := &m.topology[i]

		file, err := os.Open(t.file)
		if err != nil {
			cclog.ComponentError(
				m.name,
				fmt.Sprintf("Read(): Failed to open file '%s': %v", t.file, err))
			continue
		}
		scanner := bufio.NewScanner(file)

		// Read line by line
		for scanner.Scan() {
			line := scanner.Text()
			split := strings.Fields(line)
			if len(split) != 2 {
				continue
			}
			key := split[0]
			value, err := strconv.ParseInt(split[1], 10, 64)
			if err != nil {
				cclog.ComponentError(
					m.name,
					fmt.Sprintf("Read(): Failed to convert %s='%s' to int64: %v", key, split[1], err))
				continue
			}

			if m.config.SendAbsoluteValues {
				msg, err := lp.NewMetric(
					"numastats_"+key,
					t.tagSet,
					m.meta,
					value,
					now,
				)
				if err == nil {
					output <- msg
				}
			}

			if m.config.SendDerivedValues {
				prev, ok := t.previousValues[key]
				if ok {
					rate := float64(value-prev) / timeDiff
					msg, err := lp.NewMetric(
						"numastats_"+key+"_rate",
						t.tagSet,
						m.meta,
						rate,
						now,
					)
					if err == nil {
						output <- msg
					}
				}
				t.previousValues[key] = value
			}
		}
		file.Close()
	}
}

func (m *NUMAStatsCollector) Close() {
	m.init = false
}
