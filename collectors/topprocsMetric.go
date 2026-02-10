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
	"os/exec"
	"strings"
	"time"

	cclog "github.com/ClusterCockpit/cc-lib/v2/ccLogger"
	lp "github.com/ClusterCockpit/cc-lib/v2/ccMessage"
)

const MAX_NUM_PROCS = 10
const DEFAULT_NUM_PROCS = 2

type TopProcsCollectorConfig struct {
	Num_procs int `json:"num_procs"`
}

type TopProcsCollector struct {
	metricCollector
	tags   map[string]string
	config TopProcsCollectorConfig
}

func (m *TopProcsCollector) Init(config json.RawMessage) error {
	var err error
	m.name = "TopProcsCollector"
	m.parallel = true
	m.tags = map[string]string{
		"type": "node",
	}
	m.meta = map[string]string{
		"source": m.name,
		"group":  "TopProcs",
	}
	if len(config) > 0 {
		err = json.Unmarshal(config, &m.config)
		if err != nil {
			return fmt.Errorf("%s Init(): json.Unmarshal() failed: %w", m.name, err)
		}
	} else {
		m.config.Num_procs = int(DEFAULT_NUM_PROCS)
	}
	if m.config.Num_procs <= 0 || m.config.Num_procs > MAX_NUM_PROCS {
		return fmt.Errorf("num_procs option must be set in 'topprocs' config (range: 1-%d)", MAX_NUM_PROCS)
	}
	if err := m.setup(); err != nil {
		return fmt.Errorf("%s Init(): setup() call failed: %w", m.name, err)
	}
	command := exec.Command("ps", "-Ao", "comm", "--sort=-pcpu")
	_, err = command.Output()
	if err != nil {
		return fmt.Errorf("%s Init(): failed to get output from command: %w", m.name, err)
	}
	m.init = true
	return nil
}

func (m *TopProcsCollector) Read(interval time.Duration, output chan lp.CCMessage) {
	if !m.init {
		return
	}
	command := exec.Command("ps", "-Ao", "comm", "--sort=-pcpu")
	stdout, err := command.Output()
	if err != nil {
		cclog.ComponentError(
			m.name,
			fmt.Sprintf("Read(): Failed to read output from command \"%s\": %v", command.String(), err))
		return
	}

	lines := strings.Split(string(stdout), "\n")
	for i := 1; i < m.config.Num_procs+1; i++ {
		name := fmt.Sprintf("topproc%d", i)
		y, err := lp.NewMessage(name, m.tags, m.meta, map[string]any{"value": string(lines[i])}, time.Now())
		if err == nil {
			output <- y
		}
	}
}

func (m *TopProcsCollector) Close() {
	m.init = false
}
