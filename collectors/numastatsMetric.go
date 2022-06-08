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
)

//
// Numa policy hit/miss statistics
//
// numa_hit:
//   A process wanted to allocate memory from this node, and succeeded.
// numa_miss:
//   A process wanted to allocate memory from another node,
//   but ended up with memory from this node.
// numa_foreign:
//   A process wanted to allocate on this node,
//   but ended up with memory from another node.
// local_node:
//   A process ran on this node's CPU,
//   and got memory from this node.
// other_node:
//   A process ran on a different node's CPU
//   and got memory from this node.
// interleave_hit:
//   Interleaving wanted to allocate from this node
//   and succeeded.
//
// See: https://www.kernel.org/doc/html/latest/admin-guide/numastat.html
//
type NUMAStatsCollectorTopolgy struct {
	file   string
	tagSet map[string]string
}

type NUMAStatsCollector struct {
	metricCollector
	topology []NUMAStatsCollectorTopolgy
}

func (m *NUMAStatsCollector) Init(config json.RawMessage) error {
	// Check if already initialized
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
				file:   file,
				tagSet: map[string]string{"memoryDomain": node},
			})
	}

	m.init = true
	return nil
}

func (m *NUMAStatsCollector) Read(interval time.Duration, output chan lp.CCMetric) {
	if !m.init {
		return
	}

	for i := range m.topology {
		// Loop for all NUMA domains
		t := &m.topology[i]

		now := time.Now()
		file, err := os.Open(t.file)
		if err != nil {
			cclog.ComponentError(
				m.name,
				fmt.Sprintf("Read(): Failed to open file '%s': %v", t.file, err))
			return
		}
		scanner := bufio.NewScanner(file)

		// Read line by line
		for scanner.Scan() {
			split := strings.Fields(scanner.Text())
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
			y, err := lp.New(
				"numastats_"+key,
				t.tagSet,
				m.meta,
				map[string]interface{}{"value": value},
				now,
			)
			if err == nil {
				output <- y
			}
		}

		file.Close()
	}
}

func (m *NUMAStatsCollector) Close() {
	m.init = false
}
