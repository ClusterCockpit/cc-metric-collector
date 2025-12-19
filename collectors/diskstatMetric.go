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
	"os"
	"strings"
	"syscall"
	"time"

	cclog "github.com/ClusterCockpit/cc-lib/ccLogger"
	lp "github.com/ClusterCockpit/cc-lib/ccMessage"
)

const MOUNTFILE = `/proc/self/mounts`

type DiskstatCollectorConfig struct {
	ExcludeMetrics     []string `json:"exclude_metrics,omitempty"`
	ExcludeDevices     []string `json:"exclude_devices,omitempty"`
	ExcludeMountpoints []string `json:"exclude_mountpoints,omitempty"`
	IncludeDevices     []string `json:"include_devices,omitempty"`
	IncludeMountpoints []string `json:"include_mountpoints,omitempty"`
	UseMountpoint      bool     `json:"mountpoint_as_stype,omitempty"`
	UseIncludeConfig   bool     `json:"use_include_config,omitempty"`
}

type DiskstatCollector struct {
	metricCollector
	config             DiskstatCollectorConfig
	allowedMetrics     map[string]bool
	includeDevices     map[string]bool
	includeMountpoints map[string]bool
	excludeDevices     map[string]bool
	excludeMountpoints map[string]bool
}

func (m *DiskstatCollector) Init(config json.RawMessage) error {
	m.name = "DiskstatCollector"
	m.parallel = true
	m.meta = map[string]string{"source": m.name, "group": "Disk"}
	m.setup()
	m.config.UseIncludeConfig = false
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
	availDevices := make(map[string]struct{})
	availMpoints := make(map[string]struct{})
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) == 0 {
			continue
		}
		linefields := strings.Fields(line)
		availDevices[linefields[0]] = struct{}{}
		availMpoints[linefields[1]] = struct{}{}
	}
	m.includeDevices = make(map[string]bool)
	for _, incl := range m.config.IncludeDevices {
		if _, ok := availDevices[incl]; ok {
			m.includeDevices[incl] = true
		} else {
			cclog.ComponentWarn(m.name, "Included Mount device ", incl, " does not exist")
		}
	}
	m.includeMountpoints = make(map[string]bool)
	for _, incl := range m.config.IncludeMountpoints {
		if _, ok := availMpoints[incl]; ok {
			m.includeMountpoints[incl] = true
		} else {
			cclog.ComponentWarn(m.name, "Included Mount point ", incl, " does not exist")
		}
	}
	m.excludeMountpoints = make(map[string]bool)
	for _, excl := range m.config.ExcludeMountpoints {
		m.excludeMountpoints[excl] = true
	}
	m.excludeDevices = make(map[string]bool)
	for _, excl := range m.config.ExcludeDevices {
		m.excludeDevices[excl] = true
	}

	m.init = true
	return nil
}

func (m *DiskstatCollector) Read(interval time.Duration, output chan lp.CCMessage) {
	if !m.init {
		return
	}

	file, err := os.Open(MOUNTFILE)
	if err != nil {
		cclog.ComponentError(m.name, err.Error())
		return
	}
	defer file.Close()

	part_max_used := uint64(0)
	part_max_used_device := ""
	part_max_used_mountpoint := ""
	scanner := bufio.NewScanner(file)
mountLoop:
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) == 0 {
			continue
		}
		// if !strings.HasPrefix(line, "/dev") {
		// 	continue
		// }
		linefields := strings.Fields(line)
		if strings.Contains(linefields[0], "loop") {
			continue
		}
		if strings.Contains(linefields[1], "boot") {
			continue
		}

		mountPath := strings.Replace(linefields[1], `\040`, " ", -1)

		if m.config.UseIncludeConfig {
			_, ok1 := m.includeDevices[linefields[0]]
			_, ok2 := m.includeMountpoints[linefields[1]]
			if !(ok1 || ok2) {
				continue mountLoop
			}
		} else {
			_, ok1 := m.excludeDevices[linefields[0]]
			_, ok2 := m.excludeMountpoints[linefields[1]]
			if ok1 || ok2 {
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
		tags := map[string]string{"type": "node", "stype": "filesystem", "stype-id": linefields[0]}
		if m.config.UseMountpoint {
			tags["stype-id"] = linefields[1]
		}
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
				part_max_used_mountpoint = linefields[1]
				part_max_used_device = linefields[0]
			}
		}
	}
	if m.allowedMetrics["part_max_used"] && len(part_max_used_mountpoint) > 0 {
		tags := map[string]string{"type": "node", "stype": "filesystem", "stype-id": part_max_used_device}
		if m.config.UseMountpoint {
			tags["stype-id"] = part_max_used_mountpoint
		}
		y, err := lp.NewMessage("part_max_used", tags, m.meta, map[string]interface{}{"value": int(part_max_used)}, time.Now())
		if err == nil {
			y.AddMeta("unit", "percent")
			output <- y
		}
	}
}

func (m *DiskstatCollector) Close() {
	m.init = false
}
