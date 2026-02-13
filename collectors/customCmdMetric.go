// Copyright (C) NHR@FAU, University Erlangen-Nuremberg.
// All rights reserved. This file is part of cc-lib.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.
// additional authors:
// Holger Obermaier (NHR@KIT)

package collectors

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"slices"
	"strings"
	"time"

	cclog "github.com/ClusterCockpit/cc-lib/v2/ccLogger"
	lp "github.com/ClusterCockpit/cc-lib/v2/ccMessage"

	receivers "github.com/ClusterCockpit/cc-lib/v2/receivers"
	lp2 "github.com/influxdata/line-protocol/v2/lineprotocol"
)

const CUSTOMCMDPATH = `/home/unrz139/Work/cc-metric-collector/collectors/custom`

type CustomCmdCollectorConfig struct {
	Commands       []string `json:"commands"`
	Files          []string `json:"files"`
	ExcludeMetrics []string `json:"exclude_metrics"`
}

type CustomCmdCollector struct {
	metricCollector
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
		// Execute configured commands
		cmdFields := strings.Fields(cmd)
		command := exec.Command(cmdFields[0], cmdFields[1:]...)
		stdout, _ := command.StdoutPipe()
		errBuf := new(bytes.Buffer)
		command.Stderr = errBuf

		// Start command
		if err := command.Start(); err != nil {
			cclog.ComponentError(
				m.name,
				fmt.Sprintf("Read(): Failed to start command \"%s\": %v", command.String(), err),
			)
			return
		}

		// Read and decode influxDB line-protocol from command output
		d := lp2.NewDecoder(stdout)
		for d.Next() {
			metric, err := receivers.DecodeInfluxMessage(d)
			if err != nil {
				cclog.ComponentError(
					m.name,
					fmt.Sprintf("Read(): Failed to decode influx Message: %v", err),
				)
				continue
			}
			if slices.Contains(m.config.ExcludeMetrics, metric.Name()) {
				continue
			}
			output <- metric
		}

		// Wait for command end
		if err := command.Wait(); err != nil {
			errMsg, _ := io.ReadAll(errBuf)
			cclog.ComponentError(
				m.name,
				fmt.Sprintf("Read(): Failed to wait for the end of command \"%s\": %v\n", command.String(), err),
			)
			cclog.ComponentError(
				m.name,
				fmt.Sprintf("Read(): command stderr: \"%s\"\n", strings.TrimSpace(string(errMsg))))
			return
		}
	}
	for _, filename := range m.files {
		file, err := os.Open(filename)
		if err != nil {
			cclog.ComponentError(
				m.name,
				fmt.Sprintf("Read(): Failed to open file \"%s\": %v\n", filename, err),
			)
		}

		// Read and decode influxDB line-protocol from file
		d := lp2.NewDecoder(file)
		for d.Next() {
			metric, err := receivers.DecodeInfluxMessage(d)
			if err != nil {
				cclog.ComponentError(
					m.name,
					fmt.Sprintf("Read(): Failed to decode influx Message: %v", err),
				)
				continue
			}
			if slices.Contains(m.config.ExcludeMetrics, metric.Name()) {
				continue
			}
			output <- metric
		}

		if err := file.Close(); err != nil {
			cclog.ComponentError(
				m.name,
				fmt.Sprintf("Read(): Failed to close file \"%s\": %v\n", filename, err),
			)
		}
	}
}

func (m *CustomCmdCollector) Close() {
	m.init = false
}
