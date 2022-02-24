package collectors

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	cclog "github.com/ClusterCockpit/cc-metric-collector/internal/ccLogger"
	lp "github.com/ClusterCockpit/cc-metric-collector/internal/ccMetric"
)

const MEMSTATFILE = "/proc/meminfo"
const NUMA_MEMSTAT_BASE = "/sys/devices/system/node"

type MemstatCollectorConfig struct {
	ExcludeMetrics []string `json:"exclude_metrics"`
	NodeStats      bool     `json:"node_stats,omitempty"`
	NumaStats      bool     `json:"numa_stats,omitempty"`
}

type MemstatCollectorNode struct {
	file string
	tags map[string]string
}

type MemstatCollector struct {
	metricCollector
	stats     map[string]int64
	tags      map[string]string
	matches   map[string]string
	config    MemstatCollectorConfig
	nodefiles map[int]MemstatCollectorNode
}

func getStats(filename string) map[string]float64 {
	stats := make(map[string]float64)
	file, err := os.Open(filename)
	if err != nil {
		cclog.Error(err.Error())
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		linefields := strings.Fields(line)
		if len(linefields) == 3 {
			v, err := strconv.ParseFloat(linefields[1], 64)
			if err == nil {
				stats[strings.Trim(linefields[0], ":")] = v
			}
		} else if len(linefields) == 5 {
			v, err := strconv.ParseFloat(linefields[3], 64)
			if err == nil {
				stats[strings.Trim(linefields[0], ":")] = v
			}
		}
	}
	return stats
}

func (m *MemstatCollector) Init(config json.RawMessage) error {
	var err error
	m.name = "MemstatCollector"
	m.config.NodeStats = true
	m.config.NumaStats = false
	if len(config) > 0 {
		err = json.Unmarshal(config, &m.config)
		if err != nil {
			return err
		}
	}
	m.meta = map[string]string{"source": m.name, "group": "Memory", "unit": "kByte"}
	m.stats = make(map[string]int64)
	m.matches = make(map[string]string)
	m.tags = map[string]string{"type": "node"}
	matches := map[string]string{
		"MemTotal":     "mem_total",
		"SwapTotal":    "swap_total",
		"SReclaimable": "mem_sreclaimable",
		"Slab":         "mem_slab",
		"MemFree":      "mem_free",
		"Buffers":      "mem_buffers",
		"Cached":       "mem_cached",
		"MemAvailable": "mem_available",
		"SwapFree":     "swap_free",
		"MemShared":    "mem_shared",
	}
	for k, v := range matches {
		_, skip := stringArrayContains(m.config.ExcludeMetrics, k)
		if !skip {
			m.matches[k] = v
		}
	}
	if len(m.matches) == 0 {
		return errors.New("no metrics to collect")
	}
	m.setup()

	if m.config.NodeStats {
		if stats := getStats(MEMSTATFILE); len(stats) == 0 {
			return fmt.Errorf("cannot read data from file %s", MEMSTATFILE)
		}
	}

	if m.config.NumaStats {
		globPattern := filepath.Join(NUMA_MEMSTAT_BASE, "node[0-9]*", "meminfo")
		regex := regexp.MustCompile(filepath.Join(NUMA_MEMSTAT_BASE, "node(\\d+)", "meminfo"))
		files, err := filepath.Glob(globPattern)
		if err == nil {
			m.nodefiles = make(map[int]MemstatCollectorNode)
			for _, f := range files {
				if stats := getStats(f); len(stats) == 0 {
					return fmt.Errorf("cannot read data from file %s", f)
				}
				rematch := regex.FindStringSubmatch(f)
				if len(rematch) == 2 {
					id, err := strconv.Atoi(rematch[1])
					if err == nil {
						f := MemstatCollectorNode{
							file: f,
							tags: map[string]string{
								"type":    "memoryDomain",
								"type-id": fmt.Sprintf("%d", id),
							},
						}
						m.nodefiles[id] = f
					}
				}
			}
		}
	}
	m.init = true
	return err
}

func (m *MemstatCollector) Read(interval time.Duration, output chan lp.CCMetric) {
	if !m.init {
		return
	}

	sendStats := func(stats map[string]float64, tags map[string]string) {
		for match, name := range m.matches {
			var value float64 = 0
			if v, ok := stats[match]; ok {
				value = v
			}
			y, err := lp.New(name, tags, m.meta, map[string]interface{}{"value": value}, time.Now())
			if err == nil {
				output <- y
			}
		}
		if _, skip := stringArrayContains(m.config.ExcludeMetrics, "mem_used"); !skip {
			if freeVal, free := stats["MemFree"]; free {
				if bufVal, buffers := stats["Buffers"]; buffers {
					if cacheVal, cached := stats["Cached"]; cached {
						memUsed := stats["MemTotal"] - (freeVal + bufVal + cacheVal)
						y, err := lp.New("mem_used", tags, m.meta, map[string]interface{}{"value": memUsed}, time.Now())
						if err == nil {
							output <- y
						}
					}
				}
			}
		}
	}

	if m.config.NodeStats {
		nodestats := getStats(MEMSTATFILE)
		sendStats(nodestats, m.tags)
	}

	if m.config.NumaStats {
		for _, nodeConf := range m.nodefiles {
			stats := getStats(nodeConf.file)
			sendStats(stats, nodeConf.tags)
		}
	}
}

func (m *MemstatCollector) Close() {
	m.init = false
}
