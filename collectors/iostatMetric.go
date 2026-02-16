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
	"slices"
	"strconv"
	"strings"
	"time"

	cclog "github.com/ClusterCockpit/cc-lib/v2/ccLogger"
	lp "github.com/ClusterCockpit/cc-lib/v2/ccMessage"
)

const IOSTATFILE = `/proc/diskstats`

type IOstatCollectorConfig struct {
	ExcludeMetrics []string `json:"exclude_metrics,omitempty"`
	ExcludeDevices []string `json:"exclude_devices,omitempty"`
}

type IOstatCollectorEntry struct {
	currentValues map[string]int64
	lastValues    map[string]int64
	tags          map[string]string
}

type IOstatCollector struct {
	metricCollector

	matches map[string]int
	config  IOstatCollectorConfig
	devices map[string]IOstatCollectorEntry
}

func (m *IOstatCollector) Init(config json.RawMessage) error {
	var err error
	m.name = "IOstatCollector"
	m.parallel = true
	m.meta = map[string]string{"source": m.name, "group": "Disk"}
	if err := m.setup(); err != nil {
		return fmt.Errorf("%s Init(): setup() call failed: %w", m.name, err)
	}
	if len(config) > 0 {
		err = json.Unmarshal(config, &m.config)
		if err != nil {
			return err
		}
	}
	// https://www.kernel.org/doc/html/latest/admin-guide/iostats.html
	matches := map[string]int{
		"io_reads":             3,
		"io_reads_merged":      4,
		"io_read_sectors":      5,
		"io_read_ms":           6,
		"io_writes":            7,
		"io_writes_merged":     8,
		"io_writes_sectors":    9,
		"io_writes_ms":         10,
		"io_ioops":             11,
		"io_ioops_ms":          12,
		"io_ioops_weighted_ms": 13,
		"io_discards":          14,
		"io_discards_merged":   15,
		"io_discards_sectors":  16,
		"io_discards_ms":       17,
		"io_flushes":           18,
		"io_flushes_ms":        19,
	}
	m.devices = make(map[string]IOstatCollectorEntry)
	m.matches = make(map[string]int)
	for k, v := range matches {
		if !slices.Contains(m.config.ExcludeMetrics, k) {
			m.matches[k] = v
		}
	}
	if len(m.matches) == 0 {
		return errors.New("no metrics to collect")
	}
	file, err := os.Open(IOSTATFILE)
	if err != nil {
		return fmt.Errorf("%s Init(): Failed to open file \"%s\": %w", m.name, IOSTATFILE, err)
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		linefields := strings.Fields(line)
		if len(linefields) < 3 {
			continue
		}
		device := linefields[2]

		if strings.Contains(device, "loop") {
			continue
		}
		if slices.Contains(m.config.ExcludeDevices, device) {
			continue
		}
		currentValues := make(map[string]int64)
		lastValues := make(map[string]int64)
		for m := range m.matches {
			currentValues[m] = 0
			lastValues[m] = 0
		}
		for name, idx := range m.matches {
			if idx < len(linefields) {
				if value, err := strconv.ParseInt(linefields[idx], 0, 64); err == nil {
					currentValues[name] = value
					lastValues[name] = value // Set last to current for first read
				}
			}
		}
		m.devices[device] = IOstatCollectorEntry{
			tags: map[string]string{
				"device": device,
				"type":   "node",
			},
			currentValues: currentValues,
			lastValues:    lastValues,
		}
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("%s Init(): Failed to close file \"%s\": %w", m.name, IOSTATFILE, err)
	}

	m.init = true
	return err
}

func (m *IOstatCollector) Read(interval time.Duration, output chan lp.CCMessage) {
	if !m.init {
		return
	}

	file, err := os.Open(IOSTATFILE)
	if err != nil {
		cclog.ComponentError(
			m.name,
			fmt.Sprintf("Read(): Failed to open file '%s': %v", IOSTATFILE, err))
		return
	}
	defer func() {
		if err := file.Close(); err != nil {
			cclog.ComponentError(
				m.name,
				fmt.Sprintf("Read(): Failed to close file '%s': %v", IOSTATFILE, err))
		}
	}()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) == 0 {
			continue
		}
		linefields := strings.Fields(line)
		if len(linefields) < 3 {
			continue
		}
		device := linefields[2]
		if strings.Contains(device, "loop") {
			continue
		}
		if slices.Contains(m.config.ExcludeDevices, device) {
			continue
		}
		if _, ok := m.devices[device]; !ok {
			continue
		}
		// Update current and last values
		entry := m.devices[device]
		for name, idx := range m.matches {
			if idx < len(linefields) {
				x, err := strconv.ParseInt(linefields[idx], 0, 64)
				if err == nil {
					// Calculate difference using previous current and new value
					diff := x - entry.currentValues[name]
					y, err := lp.NewMetric(name, entry.tags, m.meta, int(diff), time.Now())
					if err == nil {
						output <- y
					}
					// Update last to previous current, and current to new value
					entry.lastValues[name] = entry.currentValues[name]
					entry.currentValues[name] = x
				}
			}
		}
		m.devices[device] = entry
	}
}

func (m *IOstatCollector) Close() {
	m.init = false
}
