// Copyright (C) NHR@FAU, University Erlangen-Nuremberg.
// All rights reserved. This file is part of cc-lib.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.
// additional authors:
// Holger Obermaier (NHR@KIT)

package collectors

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	cclog "github.com/ClusterCockpit/cc-lib/v2/ccLogger"
	lp "github.com/ClusterCockpit/cc-lib/v2/ccMessage"
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
	stats       map[string]int64
	tags        map[string]string
	matches     map[string]string
	config      MemstatCollectorConfig
	nodefiles   map[int]MemstatCollectorNode
	sendMemUsed bool
}

type MemstatStats struct {
	value float64
	unit  string
}

func getStats(filename string) map[string]MemstatStats {
	stats := make(map[string]MemstatStats)
	file, err := os.Open(filename)
	if err != nil {
		cclog.Error(err.Error())
	}
	defer func() {
		if err := file.Close(); err != nil {
			cclog.Error(err.Error())
		}
	}()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		linefields := strings.Fields(line)
		if len(linefields) == 3 {
			v, err := strconv.ParseFloat(linefields[1], 64)
			if err == nil {
				stats[strings.Trim(linefields[0], ":")] = MemstatStats{
					value: v,
					unit:  linefields[2],
				}
			}
		} else if len(linefields) == 5 {
			v, err := strconv.ParseFloat(linefields[3], 64)
			if err == nil {
				cclog.ComponentDebug("getStats", strings.Trim(linefields[2], ":"), v, linefields[4])
				stats[strings.Trim(linefields[2], ":")] = MemstatStats{
					value: v,
					unit:  linefields[4],
				}
			}
		}
	}
	return stats
}

func (m *MemstatCollector) Init(config json.RawMessage) error {
	var err error
	m.name = "MemstatCollector"
	m.parallel = true
	m.config.NodeStats = true
	m.config.NumaStats = false
	if len(config) > 0 {
		err = json.Unmarshal(config, &m.config)
		if err != nil {
			return err
		}
	}
	m.meta = map[string]string{"source": m.name, "group": "Memory"}
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
		if !slices.Contains(m.config.ExcludeMetrics, k) {
			m.matches[k] = v
		}
	}
	m.sendMemUsed = false
	if !slices.Contains(m.config.ExcludeMetrics, "mem_used") {
		m.sendMemUsed = true
	}
	if len(m.matches) == 0 {
		return errors.New("no metrics to collect")
	}
	if err := m.setup(); err != nil {
		return fmt.Errorf("%s Init(): setup() call failed: %w", m.name, err)
	}

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
								"type-id": strconv.Itoa(id),
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

func (m *MemstatCollector) Read(interval time.Duration, output chan lp.CCMessage) {
	if !m.init {
		return
	}

	sendStats := func(stats map[string]MemstatStats, tags map[string]string) {
		for match, name := range m.matches {
			var value float64 = 0
			unit := ""
			if v, ok := stats[match]; ok {
				value = v.value
				if len(v.unit) > 0 {
					unit = v.unit
				}
			}

			y, err := lp.NewMessage(name, tags, m.meta, map[string]any{"value": value}, time.Now())
			if err == nil {
				if len(unit) > 0 {
					y.AddMeta("unit", unit)
				}
				output <- y
			}
		}
		if m.sendMemUsed {
			memUsed := 0.0
			unit := ""
			if totalVal, total := stats["MemTotal"]; total {
				if freeVal, free := stats["MemFree"]; free {
					memUsed = totalVal.value - freeVal.value
					if len(totalVal.unit) > 0 {
						unit = totalVal.unit
					} else if len(freeVal.unit) > 0 {
						unit = freeVal.unit
					}
					if bufVal, buffers := stats["Buffers"]; buffers {
						memUsed -= bufVal.value
						if len(bufVal.unit) > 0 && len(unit) == 0 {
							unit = bufVal.unit
						}
						if cacheVal, cached := stats["Cached"]; cached {
							memUsed -= cacheVal.value
							if len(cacheVal.unit) > 0 && len(unit) == 0 {
								unit = cacheVal.unit
							}
						}
					}
				}
			}
			y, err := lp.NewMessage("mem_used", tags, m.meta, map[string]any{"value": memUsed}, time.Now())
			if err == nil {
				if len(unit) > 0 {
					y.AddMeta("unit", unit)
				}
				output <- y
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
