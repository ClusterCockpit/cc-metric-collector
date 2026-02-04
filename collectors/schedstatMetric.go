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

const SCHEDSTATFILE = `/proc/schedstat`

// These are the fields we read from the JSON configuration
type SchedstatCollectorConfig struct {
	ExcludeMetrics []string `json:"exclude_metrics,omitempty"`
}

// This contains all variables we need during execution and the variables
// defined by metricCollector (name, init, ...)
type SchedstatCollector struct {
	metricCollector
	config        SchedstatCollectorConfig     // the configuration structure
	lastTimestamp time.Time                    // Store time stamp of last tick to derive values
	meta          map[string]string            // default meta information
	cputags       map[string]map[string]string // default tags
	olddata       map[string]map[string]int64  // default tags
}

// Functions to implement MetricCollector interface
// Init(...), Read(...), Close()
// See: metricCollector.go

// Init initializes the sample collector
// Called once by the collector manager
// All tags, meta data tags and metrics that do not change over the runtime should be set here
func (m *SchedstatCollector) Init(config json.RawMessage) error {
	var err error = nil
	// Always set the name early in Init() to use it in cclog.Component* functions
	m.name = "SchedstatCollector"
	// This is for later use, also call it early
	m.setup()
	// Tell whether the collector should be run in parallel with others (reading files, ...)
	// or it should be run serially, mostly for collectors acutally doing measurements
	// because they should not measure the execution of the other collectors
	m.parallel = true
	// Define meta information sent with each metric
	// (Can also be dynamic or this is the basic set with extension through AddMeta())
	m.meta = map[string]string{"source": m.name, "group": "SCHEDSTAT"}

	// Read in the JSON configuration
	if len(config) > 0 {
		err = json.Unmarshal(config, &m.config)
		if err != nil {
			cclog.ComponentError(m.name, "Error reading config:", err.Error())
			return err
		}
	}

	// Check input file
	file, err := os.Open(string(SCHEDSTATFILE))
	if err != nil {
		cclog.ComponentError(m.name, err.Error())
	}
	defer file.Close()

	// Pre-generate tags for all CPUs
	num_cpus := 0
	m.cputags = make(map[string]map[string]string)
	m.olddata = make(map[string]map[string]int64)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		linefields := strings.Fields(line)
		if strings.HasPrefix(linefields[0], "cpu") && strings.Compare(linefields[0], "cpu") != 0 {
			cpustr := strings.TrimLeft(linefields[0], "cpu")
			cpu, _ := strconv.Atoi(cpustr)
			running, _ := strconv.ParseInt(linefields[7], 10, 64)
			waiting, _ := strconv.ParseInt(linefields[8], 10, 64)
			m.cputags[linefields[0]] = map[string]string{"type": "hwthread", "type-id": fmt.Sprintf("%d", cpu)}
			m.olddata[linefields[0]] = map[string]int64{"running": running, "waiting": waiting}
			num_cpus++
		}
	}

	// Save current timestamp
	m.lastTimestamp = time.Now()

	// Set this flag only if everything is initialized properly, all required files exist, ...
	m.init = true
	return err
}

func (m *SchedstatCollector) ParseProcLine(linefields []string, tags map[string]string, output chan lp.CCMessage, now time.Time, tsdelta time.Duration) {
	running, _ := strconv.ParseInt(linefields[7], 10, 64)
	waiting, _ := strconv.ParseInt(linefields[8], 10, 64)
	diff_running := running - m.olddata[linefields[0]]["running"]
	diff_waiting := waiting - m.olddata[linefields[0]]["waiting"]

	l_running := float64(diff_running) / tsdelta.Seconds() / 1000_000_000
	l_waiting := float64(diff_waiting) / tsdelta.Seconds() / 1000_000_000

	m.olddata[linefields[0]]["running"] = running
	m.olddata[linefields[0]]["waiting"] = waiting
	value := l_running + l_waiting

	y, err := lp.NewMessage("cpu_load_core", tags, m.meta, map[string]interface{}{"value": value}, now)
	if err == nil {
		// Send it to output channel
		output <- y
	}
}

// Read collects all metrics belonging to the sample collector
// and sends them through the output channel to the collector manager
func (m *SchedstatCollector) Read(interval time.Duration, output chan lp.CCMessage) {
	if !m.init {
		return
	}

	//timestamps
	now := time.Now()
	tsdelta := now.Sub(m.lastTimestamp)

	file, err := os.Open(string(SCHEDSTATFILE))
	if err != nil {
		cclog.ComponentError(m.name, err.Error())
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		linefields := strings.Fields(line)
		if strings.HasPrefix(linefields[0], "cpu") {
			m.ParseProcLine(linefields, m.cputags[linefields[0]], output, now, tsdelta)
		}
	}

	m.lastTimestamp = now

}

// Close metric collector: close network connection, close files, close libraries, ...
// Called once by the collector manager
func (m *SchedstatCollector) Close() {
	// Unset flag
	m.init = false
}
