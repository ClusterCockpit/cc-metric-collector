// Copyright (C) NHR@FAU, University Erlangen-Nuremberg.
// All rights reserved. This file is part of cc-lib.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.
// additional authors:
// Holger Obermaier (NHR@KIT)

package collectors

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"slices"
	"strings"
	"time"

	cclog "github.com/ClusterCockpit/cc-lib/v2/ccLogger"
	lp "github.com/ClusterCockpit/cc-lib/v2/ccMessage"
	influx "github.com/influxdata/line-protocol"
)

const CUSTOMCMDPATH = `/home/unrz139/Work/cc-metric-collector/collectors/custom`

type CustomCmdCollectorConfig struct {
	Commands       []string `json:"commands"`
	Files          []string `json:"files"`
	ExcludeMetrics []string `json:"exclude_metrics"`
}

type CustomCmdCollector struct {
	metricCollector
	handler  *influx.MetricHandler
	parser   *influx.Parser
	config   CustomCmdCollectorConfig
	commands []string
	files    []string
}

func (m *CustomCmdCollector) Init(config json.RawMessage) error {
	var err error
	m.name = "CustomCmdCollector"
	m.parallel = true
	m.meta = map[string]string{
		"source": m.name,
		"group":  "Custom",
	}
	if len(config) > 0 {
		err = json.Unmarshal(config, &m.config)
		if err != nil {
			return fmt.Errorf("%s Init(): json.Unmarshal() call failed: %w", m.name, err)
		}
	}
	if err := m.setup(); err != nil {
		return fmt.Errorf("%s Init(): setup() call failed: %w", m.name, err)
	}
	for _, c := range m.config.Commands {
		cmdfields := strings.Fields(c)
		command := exec.Command(cmdfields[0], cmdfields[1:]...)
		_, err = command.Output()
		if err == nil {
			m.commands = append(m.commands, c)
		} else {
			cclog.ComponentWarn(
				m.name,
				fmt.Sprintf("%s Init(): Execution of command \"%s\" failed: %v", m.name, command.String(), err),
			)
			continue
		}
	}
	for _, f := range m.config.Files {
		_, err = os.ReadFile(f)
		if err == nil {
			m.files = append(m.files, f)
		} else {
			cclog.ComponentWarn(
				m.name,
				fmt.Sprintf("%s Init(): Reading of file \"%s\" failed: %v", m.name, f, err),
			)
			continue
		}
	}
	if len(m.files) == 0 && len(m.commands) == 0 {
		return errors.New("no metrics to collect")
	}
	m.handler = influx.NewMetricHandler()
	m.parser = influx.NewParser(m.handler)
	m.parser.SetTimeFunc(DefaultTime)
	m.init = true
	return nil
}

var DefaultTime = func() time.Time {
	return time.Unix(42, 0)
}

func (m *CustomCmdCollector) Read(interval time.Duration, output chan lp.CCMessage) {
	if !m.init {
		return
	}
	for _, cmd := range m.commands {
		cmdfields := strings.Fields(cmd)
		command := exec.Command(cmdfields[0], cmdfields[1:]...)
		if err := command.Wait(); err != nil {
			log.Print(err)
			continue
		}
		stdout, err := command.Output()
		if err != nil {
			log.Print(err)
			continue
		}
		cmdmetrics, err := m.parser.Parse(stdout)
		if err != nil {
			log.Print(err)
			continue
		}
		for _, c := range cmdmetrics {
			if slices.Contains(m.config.ExcludeMetrics, c.Name()) {
				continue
			}

			output <- lp.FromInfluxMetric(c)
		}
	}
	for _, file := range m.files {
		buffer, err := os.ReadFile(file)
		if err != nil {
			log.Print(err)
			return
		}
		fmetrics, err := m.parser.Parse(buffer)
		if err != nil {
			log.Print(err)
			continue
		}
		for _, f := range fmetrics {
			if slices.Contains(m.config.ExcludeMetrics, f.Name()) {
				continue
			}
			output <- lp.FromInfluxMetric(f)
		}
	}
}

func (m *CustomCmdCollector) Close() {
	m.init = false
}
