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
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"

	cclog "github.com/ClusterCockpit/cc-lib/v2/ccLogger"
	lp "github.com/ClusterCockpit/cc-lib/v2/ccMessage"
)

const MOUNTFILE = `/proc/self/mounts`

type DiskstatCollectorConfig struct {
	ExcludeMetrics []string `json:"exclude_metrics,omitempty"`
	ExcludeMounts  []string `json:"exclude_mounts,omitempty"`
}

type DiskstatCollector struct {
	metricCollector
	config         DiskstatCollectorConfig
	allowedMetrics map[string]bool
}

func (m *DiskstatCollector) Init(config json.RawMessage) error {
	m.name = "DiskstatCollector"
	m.parallel = true
	m.meta = map[string]string{"source": m.name, "group": "Disk"}
	if err := m.setup(); err != nil {
		return fmt.Errorf("%s Init(): setup() call failed: %w", m.name, err)
	}
	if len(config) > 0 {
		if err := json.Unmarshal(config, &m.config); err != nil {
			return err
		}
	}
	m.allowedMetrics = map[string]bool{
		"disk_total":    true,
		"disk_free":     true,
		"part_max_used": true,
	}
	for _, excl := range m.config.ExcludeMetrics {
		if _, ok := m.allowedMetrics[excl]; ok {
			m.allowedMetrics[excl] = false
		}
	}
	file, err := os.Open(MOUNTFILE)
	if err != nil {
		cclog.ComponentError(m.name, err.Error())
		return err
	}
	defer file.Close()
	m.init = true
	return nil
}

func (m *DiskstatCollector) Read(interval time.Duration, output chan lp.CCMessage) {
	if !m.init {
		return
	}

	file, err := os.Open(MOUNTFILE)
	if err != nil {
		cclog.ComponentError(
			m.name,
			fmt.Sprintf("Read(): Failed to open file '%s': %v", MOUNTFILE, err))
		return
	}
	defer func() {
		if err := file.Close(); err != nil {
			cclog.ComponentError(
				m.name,
				fmt.Sprintf("Read(): Failed to close file '%s': %v", MOUNTFILE, err))
		}
	}()

	part_max_used := uint64(0)
	scanner := bufio.NewScanner(file)
mountLoop:
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) == 0 {
			continue
		}
		if !strings.HasPrefix(line, "/dev") {
			continue
		}
		linefields := strings.Fields(line)
		if strings.Contains(linefields[0], "loop") {
			continue
		}
		if strings.Contains(linefields[1], "boot") {
			continue
		}

		mountPath := strings.ReplaceAll(linefields[1], `\040`, " ")

		for _, excl := range m.config.ExcludeMounts {
			if strings.Contains(mountPath, excl) {
				continue mountLoop
			}
		}

		stat := syscall.Statfs_t{}
		err := syscall.Statfs(mountPath, &stat)
		if err != nil {
			continue
		}
		if stat.Blocks == 0 || stat.Bsize == 0 {
			continue
		}
		tags := map[string]string{"type": "node", "device": linefields[0]}
		total := (stat.Blocks * uint64(stat.Bsize)) / uint64(1000000000)
		if m.allowedMetrics["disk_total"] {
			y, err := lp.NewMessage("disk_total", tags, m.meta, map[string]interface{}{"value": total}, time.Now())
			if err == nil {
				y.AddMeta("unit", "GBytes")
				output <- y
			}
		}
		free := (stat.Bfree * uint64(stat.Bsize)) / uint64(1000000000)
		if m.allowedMetrics["disk_free"] {
			y, err := lp.NewMessage("disk_free", tags, m.meta, map[string]interface{}{"value": free}, time.Now())
			if err == nil {
				y.AddMeta("unit", "GBytes")
				output <- y
			}
		}
		if total > 0 {
			perc := (100 * (total - free)) / total
			if perc > part_max_used {
				part_max_used = perc
			}
		}
	}
	if m.allowedMetrics["part_max_used"] {
		y, err := lp.NewMessage("part_max_used", map[string]string{"type": "node"}, m.meta, map[string]interface{}{"value": int(part_max_used)}, time.Now())
		if err == nil {
			y.AddMeta("unit", "percent")
			output <- y
		}
	}
}

func (m *DiskstatCollector) Close() {
	m.init = false
}
