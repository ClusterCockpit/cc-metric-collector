// Copyright (C) NHR@FAU, University Erlangen-Nuremberg.
// All rights reserved. This file is part of cc-lib.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.
// additional authors:
// Holger Obermaier (NHR@KIT)

package collectors

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	cclog "github.com/ClusterCockpit/cc-lib/v2/ccLogger"
	lp "github.com/ClusterCockpit/cc-lib/v2/ccMessage"
)

// LoadavgCollector collects:
// * load average of last 1, 5 & 15 minutes
// * number of processes currently runnable
// * total number of processes in system
//
// See: https://www.kernel.org/doc/html/latest/filesystems/proc.html
const LOADAVGFILE = "/proc/loadavg"

type LoadavgCollector struct {
	metricCollector

	tags         map[string]string
	load_matches []string
	load_skips   []bool
	proc_matches []string
	proc_skips   []bool
	config       struct {
		ExcludeMetrics []string `json:"exclude_metrics,omitempty"`
	}
}

func (m *LoadavgCollector) Init(config json.RawMessage) error {
	m.name = "LoadavgCollector"
	m.parallel = true
	if err := m.setup(); err != nil {
		return fmt.Errorf("%s Init(): setup() call failed: %w", m.name, err)
	}
	if len(config) > 0 {
		err := json.Unmarshal(config, &m.config)
		if err != nil {
			return err
		}
	}
	m.meta = map[string]string{
		"source": m.name,
		"group":  "LOAD",
	}
	m.tags = map[string]string{"type": "node"}
	m.load_matches = []string{
		"load_one",
		"load_five",
		"load_fifteen",
	}
	m.load_skips = make([]bool, len(m.load_matches))
	m.proc_matches = []string{
		"proc_run",
		"proc_total",
	}
	m.proc_skips = make([]bool, len(m.proc_matches))

	for i, name := range m.load_matches {
		m.load_skips[i] = slices.Contains(m.config.ExcludeMetrics, name)
	}
	for i, name := range m.proc_matches {
		m.proc_skips[i] = slices.Contains(m.config.ExcludeMetrics, name)
	}
	m.init = true
	return nil
}

func (m *LoadavgCollector) Read(interval time.Duration, output chan lp.CCMessage) {
	if !m.init {
		return
	}
	buffer, err := os.ReadFile(LOADAVGFILE)
	if err != nil {
		cclog.ComponentError(
			m.name,
			fmt.Sprintf("Read(): Failed to read file '%s': %v", LOADAVGFILE, err))
		return
	}
	now := time.Now()

	// Load metrics
	ls := strings.Split(string(buffer), ` `)
	for i, name := range m.load_matches {
		x, err := strconv.ParseFloat(ls[i], 64)
		if err != nil {
			cclog.ComponentError(
				m.name,
				fmt.Sprintf("Read(): Failed to convert '%s' to float64: %v", ls[i], err))
			continue
		}
		if m.load_skips[i] {
			continue
		}
		y, err := lp.NewMessage(name, m.tags, m.meta, map[string]any{"value": x}, now)
		if err == nil {
			output <- y
		}
	}

	// Process metrics
	lv := strings.Split(ls[3], `/`)
	for i, name := range m.proc_matches {
		x, err := strconv.ParseInt(lv[i], 10, 64)
		if err != nil {
			cclog.ComponentError(
				m.name,
				fmt.Sprintf("Read(): Failed to convert '%s' to float64: %v", lv[i], err))
			continue
		}
		if m.proc_skips[i] {
			continue
		}
		y, err := lp.NewMessage(name, m.tags, m.meta, map[string]any{"value": x}, now)
		if err == nil {
			output <- y
		}

	}
}

func (m *LoadavgCollector) Close() {
	m.init = false
}
