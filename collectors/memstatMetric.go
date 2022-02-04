package collectors

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	cclog "github.com/ClusterCockpit/cc-metric-collector/internal/ccLogger"
	lp "github.com/ClusterCockpit/cc-metric-collector/internal/ccMetric"
)

const MEMSTATFILE = `/proc/meminfo`
const NUMADIR = `/sys/devices/system/node`

type MemstatCollectorConfig struct {
	ExcludeMetrics []string `json:"exclude_metrics,omitempty"`
	NodeStats      bool     `json:"node_stats,omitempty"`
	NumaStats      bool     `json:"numa_stats,omitempty"`
}

type MemstatCollector struct {
	metricCollector
	tags      map[string]string
	matches   map[string]string
	config    MemstatCollectorConfig
	numafiles map[int]string
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
	if (!m.config.NodeStats) && (!m.config.NumaStats) {
		return errors.New("either node_stats or numa_stats needs to be true")
	}
	m.meta = map[string]string{"source": m.name, "group": "Memory", "unit": "kByte"}
	m.numafiles = make(map[int]string)
	m.matches = make(map[string]string)
	m.tags = map[string]string{"type": "node"}
	matches := map[string]string{`MemTotal`: "mem_total",
		"SwapTotal":    "swap_total",
		"SReclaimable": "mem_sreclaimable",
		"Slab":         "mem_slab",
		"MemFree":      "mem_free",
		"Buffers":      "mem_buffers",
		"Cached":       "mem_cached",
		"MemAvailable": "mem_available",
		"SwapFree":     "swap_free"}
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
	sysInit := false
	numaInit := false
	if m.config.NodeStats {
		_, err := ioutil.ReadFile(string(MEMSTATFILE))
		if err != nil {
			return err
		}
		sysInit = true
	}
	if m.config.NumaStats {
		globPattern := filepath.Join(NUMADIR, "node*", "meminfo")
		regex := regexp.MustCompile(`node(\d+)`)
		numafiles, err := filepath.Glob(globPattern)
		if err == nil {
			for _, f := range numafiles {
				_, err := ioutil.ReadFile(f)
				if err != nil {
					cclog.ComponentError(m.name, "Skipping NUMA meminfo file:", f)
					continue
				}
				splitPath := strings.Split(f, "/")
				if regex.MatchString(splitPath[5]) {
					rematch := regex.FindStringSubmatch(splitPath[5])
					if len(rematch) == 2 {
						nodeid, err := strconv.Atoi(rematch[1])
						if err == nil {
							m.numafiles[nodeid] = f
						}
					}
				}

			}
		}
		if len(m.numafiles) > 0 {
			numaInit = true
		}
	}
	if sysInit || numaInit {
		m.init = true
	}
	return err
}

func readMemstatRaw(filename string, re string, translate map[string]string) map[string]int64 {
	stats := make(map[string]int64)
	regex, err := regexp.Compile(re)
	if err != nil {
		return stats
	}
	buffer, err := ioutil.ReadFile(filename)
	if err != nil {
		return stats
	}

	for _, line := range strings.Split(string(buffer), "\n") {
		if regex.MatchString(line) {
			matches := regex.FindStringSubmatch(line)
			// FindStringSubmatch returns full match in index 0
			if len(matches) == 3 {
				name := string(matches[1])
				if _, ok := translate[name]; ok {
					v, err := strconv.ParseInt(string(matches[2]), 0, 64)
					if err == nil {
						stats[name] = v
					}
				}
			}
		}
	}
	if _, exists := stats[`MemTotal`]; !exists {
		return make(map[string]int64)
	}
	return stats
}

func readMemstatFile(translate map[string]string) map[string]int64 {
	return readMemstatRaw(string(MEMSTATFILE), `^([\w\(\)]+):\s*(\d+)`, translate)
}

func readNumaMemstatFile(filename string, translate map[string]string) map[string]int64 {
	return readMemstatRaw(filename, `^Node\s+\d+\s+([\w\(\)]+):\s*(\d+)`, translate)
}

func sendMatches(stats map[string]int64, matches map[string]string, tags map[string]string, meta map[string]string, output chan lp.CCMetric) {
	for raw, name := range matches {
		if value, ok := stats[raw]; ok {
			y, err := lp.New(name, tags, meta, map[string]interface{}{"value": int(float64(value) * 1.0e-3)}, time.Now())
			if err == nil {
				output <- y
			}
		}
	}
}

func sendMemUsed(stats map[string]int64, tags map[string]string, meta map[string]string, output chan lp.CCMetric) {
	if _, free := stats[`MemFree`]; free {
		if _, buffers := stats[`Buffers`]; buffers {
			if _, cached := stats[`Cached`]; cached {
				memUsed := stats[`MemTotal`] - (stats[`MemFree`] + stats[`Buffers`] + stats[`Cached`])
				y, err := lp.New("mem_used", tags, meta, map[string]interface{}{"value": int(float64(memUsed) * 1.0e-3)}, time.Now())
				if err == nil {
					output <- y
				}
			}
		}
	}
}

func sendMemShared(stats map[string]int64, tags map[string]string, meta map[string]string, output chan lp.CCMetric) {
	if _, found := stats[`MemShared`]; found {
		y, err := lp.New("mem_shared", tags, meta, map[string]interface{}{"value": int(float64(stats[`MemShared`]) * 1.0e-3)}, time.Now())
		if err == nil {
			output <- y
		}
	}
}

func (m *MemstatCollector) Read(interval time.Duration, output chan lp.CCMetric) {
	if !m.init {
		return
	}

	if m.config.NodeStats {
		cclog.ComponentDebug(m.name, "Read", string(MEMSTATFILE))
		stats := readMemstatFile(m.matches)
		sendMatches(stats, m.matches, m.tags, m.meta, output)
		if _, skip := stringArrayContains(m.config.ExcludeMetrics, "mem_used"); !skip {
			sendMemUsed(stats, m.tags, m.meta, output)
		}
		if _, skip := stringArrayContains(m.config.ExcludeMetrics, "mem_shared"); !skip {
			sendMemShared(stats, m.tags, m.meta, output)
		}

	}

	if m.config.NumaStats {
		tags := make(map[string]string)
		for k, v := range m.tags {
			tags[k] = v
		}
		tags["type"] = "memoryDomain"

		for nodeid, file := range m.numafiles {
			cclog.ComponentDebug(m.name, "Read", file)
			tags["type-id"] = fmt.Sprintf("%d", nodeid)
			stats := readNumaMemstatFile(file, m.matches)
			cclog.ComponentDebug(m.name, stats)
			sendMatches(stats, m.matches, tags, m.meta, output)
			if _, skip := stringArrayContains(m.config.ExcludeMetrics, "mem_used"); !skip {
				sendMemUsed(stats, tags, m.meta, output)
			}
			if _, skip := stringArrayContains(m.config.ExcludeMetrics, "mem_shared"); !skip {
				sendMemShared(stats, tags, m.meta, output)
			}
		}
	}
}

func (m *MemstatCollector) Close() {
	m.init = false
}
