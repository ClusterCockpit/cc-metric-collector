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
	"strconv"
	"strings"
	"time"

	cclog "github.com/ClusterCockpit/cc-lib/v2/ccLogger"
	lp "github.com/ClusterCockpit/cc-lib/v2/ccMessage"
)

// CPUFreqCollector
// a metric collector to measure the current frequency of the CPUs
// as obtained from /proc/cpuinfo
// Only measure on the first hyperthread
type CPUFreqCpuInfoCollectorTopology struct {
	isHT   bool
	tagSet map[string]string
}

type CPUFreqCpuInfoCollector struct {
	metricCollector
	topology []CPUFreqCpuInfoCollectorTopology
}

func (m *CPUFreqCpuInfoCollector) Init(_ json.RawMessage) error {
	// Check if already initialized
	if m.init {
		return nil
	}

	m.name = "CPUFreqCpuInfoCollector"
	if err := m.setup(); err != nil {
		return fmt.Errorf("%s Init(): setup() call failed: %w", m.name, err)
	}
	m.parallel = true
	m.meta = map[string]string{
		"source": m.name,
		"group":  "CPU",
		"unit":   "MHz",
	}

	const cpuInfoFile = "/proc/cpuinfo"
	file, err := os.Open(cpuInfoFile)
	if err != nil {
		return fmt.Errorf("%s Init(): failed to open file '%s': %w", m.name, cpuInfoFile, err)
	}

	// Collect topology information from file cpuinfo
	foundFreq := false
	processor := ""
	coreID := ""
	physicalPackageID := ""
	m.topology = make([]CPUFreqCpuInfoCollectorTopology, 0)
	coreSeenBefore := make(map[string]bool)

	// Read cpuinfo file, line by line
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lineSplit := strings.Split(scanner.Text(), ":")
		if len(lineSplit) == 2 {
			key := strings.TrimSpace(lineSplit[0])
			value := strings.TrimSpace(lineSplit[1])
			switch key {
			case "cpu MHz":
				// frequency
				foundFreq = true
			case "processor":
				processor = value
			case "core id":
				coreID = value
			case "physical id":
				physicalPackageID = value
			}
		}

		if err := file.Close(); err != nil {
			return fmt.Errorf("%s Init(): Call to file.Close() failed: %w", m.name, err)
		}

		// were all topology information collected?
		if foundFreq &&
			len(processor) > 0 &&
			len(coreID) > 0 &&
			len(physicalPackageID) > 0 {

			globalID := physicalPackageID + ":" + coreID

			// store collected topology information
			m.topology = append(m.topology,
				CPUFreqCpuInfoCollectorTopology{
					isHT: coreSeenBefore[globalID],
					tagSet: map[string]string{
						"type":       "hwthread",
						"type-id":    processor,
						"package_id": physicalPackageID,
					},
				},
			)

			// mark core as seen before
			coreSeenBefore[globalID] = true

			// reset topology information
			foundFreq = false
			processor = ""
			coreID = ""
			physicalPackageID = ""
		}
	}

	// Check if at least one CPU with frequency information was detected
	if len(m.topology) == 0 {
		return fmt.Errorf("%s Init(): no CPU frequency info found in %s", m.name, cpuInfoFile)
	}

	m.init = true
	return nil
}

func (m *CPUFreqCpuInfoCollector) Read(interval time.Duration, output chan lp.CCMessage) {
	// Check if already initialized
	if !m.init {
		return
	}

	const cpuInfoFile = "/proc/cpuinfo"
	file, err := os.Open(cpuInfoFile)
	if err != nil {
		cclog.ComponentError(
			m.name,
			fmt.Sprintf("Read(): Failed to open file '%s': %v", cpuInfoFile, err))
		return
	}
	defer func() {
		if err := file.Close(); err != nil {
			cclog.ComponentError(
				m.name,
				fmt.Sprintf("Read(): Failed to close file '%s': %v", cpuInfoFile, err))
		}
	}()

	processorCounter := 0
	now := time.Now()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lineSplit := strings.Split(scanner.Text(), ":")
		if len(lineSplit) == 2 {
			key := strings.TrimSpace(lineSplit[0])

			// frequency
			if key == "cpu MHz" {
				t := m.topology[processorCounter]
				if !t.isHT {
					value, err := strconv.ParseFloat(strings.TrimSpace(lineSplit[1]), 64)
					if err != nil {
						cclog.ComponentError(
							m.name,
							fmt.Sprintf("Read(): Failed to convert cpu MHz '%s' to float64: %v", lineSplit[1], err))
						return
					}
					if y, err := lp.NewMessage("cpufreq", t.tagSet, m.meta, map[string]any{"value": value}, now); err == nil {
						output <- y
					}
				}
				processorCounter++
			}
		}
	}
}

func (m *CPUFreqCpuInfoCollector) Close() {
	m.init = false
}
