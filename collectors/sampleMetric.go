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
	"time"

	cclog "github.com/ClusterCockpit/cc-lib/v2/ccLogger"
	lp "github.com/ClusterCockpit/cc-lib/v2/ccMessage"
)

// These are the fields we read from the JSON configuration
type SampleCollectorConfig struct {
	Interval string `json:"interval"`
}

// This contains all variables we need during execution and the variables
// defined by metricCollector (name, init, ...)
type SampleCollector struct {
	metricCollector
	config SampleCollectorConfig // the configuration structure
	meta   map[string]string     // default meta information
	tags   map[string]string     // default tags
}

// Functions to implement MetricCollector interface
// Init(...), Read(...), Close()
// See: metricCollector.go

// Init initializes the sample collector
// Called once by the collector manager
// All tags, meta data tags and metrics that do not change over the runtime should be set here
func (m *SampleCollector) Init(config json.RawMessage) error {
	var err error = nil
	// Always set the name early in Init() to use it in cclog.Component* functions
	m.name = "SampleCollector"
	// This is for later use, also call it early
	if err := m.setup(); err != nil {
		return fmt.Errorf("%s Init(): setup() call failed: %w", m.name, err)
	}
	// Tell whether the collector should be run in parallel with others (reading files, ...)
	// or it should be run serially, mostly for collectors actually doing measurements
	// because they should not measure the execution of the other collectors
	m.parallel = true
	// Define meta information sent with each metric
	// (Can also be dynamic or this is the basic set with extension through AddMeta())
	m.meta = map[string]string{"source": m.name, "group": "SAMPLE"}
	// Define tags sent with each metric
	// The 'type' tag is always needed, it defines the granularity of the metric
	// node -> whole system
	// socket -> CPU socket (requires socket ID as 'type-id' tag)
	// die -> CPU die (requires CPU die ID as 'type-id' tag)
	// memoryDomain -> NUMA domain (requires NUMA domain ID as 'type-id' tag)
	// llc -> Last level cache (requires last level cache ID as 'type-id' tag)
	// core -> single CPU core that may consist of multiple hardware threads (SMT) (requires core ID as 'type-id' tag)
	// hwthtread -> single CPU hardware thread (requires hardware thread ID as 'type-id' tag)
	// accelerator -> A accelerator device like GPU or FPGA (requires an accelerator ID as 'type-id' tag)
	m.tags = map[string]string{"type": "node"}
	// Read in the JSON configuration
	if len(config) > 0 {
		err = json.Unmarshal(config, &m.config)
		if err != nil {
			cclog.ComponentError(m.name, "Error reading config:", err.Error())
			return err
		}
	}

	// Set up everything that the collector requires during the Read() execution
	// Check files required, test execution of some commands, create data structure
	// for all topological entities (sockets, NUMA domains, ...)
	// Return some useful error message in case of any failures

	// Set this flag only if everything is initialized properly, all required files exist, ...
	m.init = true
	return err
}

// Read collects all metrics belonging to the sample collector
// and sends them through the output channel to the collector manager
func (m *SampleCollector) Read(interval time.Duration, output chan lp.CCMessage) {
	// Create a sample metric
	timestamp := time.Now()

	value := 1.0
	// If you want to measure something for a specific amount of time, use interval
	// start := readState()
	// time.Sleep(interval)
	// stop := readState()
	// value = (stop - start) / interval.Seconds()

	y, err := lp.NewMessage("sample_metric", m.tags, m.meta, map[string]interface{}{"value": value}, timestamp)
	if err == nil {
		// Send it to output channel
		output <- y
	}

}

// Close metric collector: close network connection, close files, close libraries, ...
// Called once by the collector manager
func (m *SampleCollector) Close() {
	// Unset flag
	m.init = false
}
