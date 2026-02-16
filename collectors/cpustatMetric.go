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
	"slices"
	"strconv"
	"strings"
	"time"

	cclog "github.com/ClusterCockpit/cc-lib/v2/ccLogger"
	lp "github.com/ClusterCockpit/cc-lib/v2/ccMessage"
	sysconf "github.com/tklauser/go-sysconf"
)

const CPUSTATFILE = `/proc/stat`

type CpustatCollectorConfig struct {
	ExcludeMetrics []string `json:"exclude_metrics,omitempty"`
}

type CpustatCollector struct {
	metricCollector

	config        CpustatCollectorConfig
	lastTimestamp time.Time // Store time stamp of last tick to derive values
	matches       map[string]int
	cputags       map[string]map[string]string
	nodetags      map[string]string
	olddata       map[string]map[string]int64
}

func (m *CpustatCollector) Init(config json.RawMessage) error {
	m.name = "CpustatCollector"
	if err := m.setup(); err != nil {
		return fmt.Errorf("%s Init(): setup() call failed: %w", m.name, err)
	}
	m.parallel = true
	m.meta = map[string]string{
		"source": m.name,
		"group":  "CPU",
	}
	m.nodetags = map[string]string{
		"type": "node",
	}
	if len(config) > 0 {
		err := json.Unmarshal(config, &m.config)
		if err != nil {
			return err
		}
	}
	matches := map[string]int{
		"cpu_user":       1,
		"cpu_nice":       2,
		"cpu_system":     3,
		"cpu_idle":       4,
		"cpu_iowait":     5,
		"cpu_irq":        6,
		"cpu_softirq":    7,
		"cpu_steal":      8,
		"cpu_guest":      9,
		"cpu_guest_nice": 10,
	}

	m.matches = make(map[string]int)
	for match, index := range matches {
		if !slices.Contains(m.config.ExcludeMetrics, match) {
			m.matches[match] = index
		}
	}

	// Check input file
	file, err := os.Open(string(CPUSTATFILE))
	if err != nil {
		cclog.ComponentError(
			m.name,
			fmt.Sprintf("Init(): Failed to open file '%s': %v", string(CPUSTATFILE), err))
	}
	defer func() {
		if err := file.Close(); err != nil {
			cclog.ComponentError(
				m.name,
				fmt.Sprintf("Init(): Failed to close file '%s': %v", string(CPUSTATFILE), err))
		}
	}()

	// Pre-generate tags for all CPUs
	num_cpus := 0
	m.cputags = make(map[string]map[string]string)
	m.olddata = make(map[string]map[string]int64)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		linefields := strings.Fields(line)
		if strings.Compare(linefields[0], "cpu") == 0 {
			m.olddata["cpu"] = make(map[string]int64)
			for k, v := range m.matches {
				m.olddata["cpu"][k], _ = strconv.ParseInt(linefields[v], 0, 64)
			}
		} else if strings.HasPrefix(linefields[0], "cpu") && strings.Compare(linefields[0], "cpu") != 0 {
			cpustr := strings.TrimLeft(linefields[0], "cpu")
			cpu, _ := strconv.Atoi(cpustr)
			m.cputags[linefields[0]] = map[string]string{
				"type":    "hwthread",
				"type-id": strconv.Itoa(cpu)}
			m.olddata[linefields[0]] = make(map[string]int64)
			for k, v := range m.matches {
				m.olddata[linefields[0]][k], _ = strconv.ParseInt(linefields[v], 0, 64)
			}
			num_cpus++
		}
	}
	m.lastTimestamp = time.Now()
	m.init = true
	return nil
}

func (m *CpustatCollector) parseStatLine(linefields []string, tags map[string]string, output chan lp.CCMessage, now time.Time, tsdelta time.Duration) {
	values := make(map[string]float64)
	clktck, _ := sysconf.Sysconf(sysconf.SC_CLK_TCK)
	for match, index := range m.matches {
		if len(match) > 0 {
			x, err := strconv.ParseInt(linefields[index], 0, 64)
			if err == nil {
				vdiff := x - m.olddata[linefields[0]][match]
				m.olddata[linefields[0]][match] = x // Store new value for next run
				values[match] = float64(vdiff) / float64(tsdelta.Seconds()) / float64(clktck)
			}
		}
	}

	sum := float64(0)
	for name, value := range values {
		sum += value
		y, err := lp.NewMessage(name, tags, m.meta, map[string]any{"value": value * 100}, now)
		if err == nil {
			y.AddTag("unit", "Percent")
			output <- y
		}
	}
	if v, ok := values["cpu_idle"]; ok {
		sum -= v
		y, err := lp.NewMessage("cpu_used", tags, m.meta, map[string]any{"value": sum * 100}, now)
		if err == nil {
			y.AddTag("unit", "Percent")
			output <- y
		}
	}
}

func (m *CpustatCollector) Read(interval time.Duration, output chan lp.CCMessage) {
	if !m.init {
		return
	}
	num_cpus := 0
	now := time.Now()
	tsdelta := now.Sub(m.lastTimestamp)

	file, err := os.Open(string(CPUSTATFILE))
	if err != nil {
		cclog.ComponentError(
			m.name,
			fmt.Sprintf("Read(): Failed to open file '%s': %v", string(CPUSTATFILE), err))
	}
	defer func() {
		if err := file.Close(); err != nil {
			cclog.ComponentError(
				m.name,
				fmt.Sprintf("Read(): Failed to close file '%s': %v", string(CPUSTATFILE), err))
		}
	}()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		linefields := strings.Fields(line)
		if strings.Compare(linefields[0], "cpu") == 0 {
			m.parseStatLine(linefields, m.nodetags, output, now, tsdelta)
		} else if strings.HasPrefix(linefields[0], "cpu") {
			m.parseStatLine(linefields, m.cputags[linefields[0]], output, now, tsdelta)
			num_cpus++
		}
	}

	num_cpus_metric, err := lp.NewMessage("num_cpus",
		m.nodetags,
		m.meta,
		map[string]any{"value": num_cpus},
		now,
	)
	if err == nil {
		output <- num_cpus_metric
	}

	m.lastTimestamp = now
}

func (m *CpustatCollector) Close() {
	m.init = false
}
