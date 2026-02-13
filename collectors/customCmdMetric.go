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
	config         CustomCmdCollectorConfig
	cmdFieldsSlice [][]string
	files          []string
}

func (m *CustomCmdCollector) Init(config json.RawMessage) error {
	m.name = "CustomCmdCollector"
	m.parallel = true
	m.meta = map[string]string{
		"source": m.name,
		"group":  "Custom",
	}

	// Read configuration
	if len(config) > 0 {
		if err := json.Unmarshal(config, &m.config); err != nil {
			return fmt.Errorf("%s Init(): json.Unmarshal() call failed: %w", m.name, err)
		}
	}

	// Setup
	if err := m.setup(); err != nil {
		return fmt.Errorf("%s Init(): setup() call failed: %w", m.name, err)
	}

	// Check if command can be executed
	for _, c := range m.config.Commands {
		cmdFields := strings.Fields(c)
		command := exec.Command(cmdFields[0], cmdFields[1:]...)
		if _, err := command.Output(); err != nil {
			cclog.ComponentWarn(
				m.name,
				fmt.Sprintf("%s Init(): Execution of command \"%s\" failed: %v", m.name, command.String(), err))
			continue
		}
		m.cmdFieldsSlice = append(m.cmdFieldsSlice, cmdFields)
	}

	// Check if file can be read
	for _, fileName := range m.config.Files {
		if _, err := os.ReadFile(fileName); err != nil {
			cclog.ComponentWarn(
				m.name,
				fmt.Sprintf("%s Init(): Reading of file \"%s\" failed: %v", m.name, fileName, err))
			continue
		}
		m.files = append(m.files, fileName)
	}

	if len(m.files) == 0 && len(m.cmdFieldsSlice) == 0 {
		return errors.New("no metrics to collect")
	}
	m.init = true
	return nil
}

func (m *CustomCmdCollector) Read(interval time.Duration, output chan lp.CCMessage) {
	if !m.init {
		return
	}

	// Execute configured commands
	for _, cmdFields := range m.cmdFieldsSlice {
		command := exec.Command(cmdFields[0], cmdFields[1:]...)
		stdout, err := command.StdoutPipe()
		if err != nil {
			cclog.ComponentError(
				m.name,
				fmt.Sprintf("Read(): Failed to create stdout pipe for command \"%s\": %v", command.String(), err),
			)
			continue
		}
		errBuf := new(bytes.Buffer)
		command.Stderr = errBuf

		// Start command
		if err := command.Start(); err != nil {
			cclog.ComponentError(
				m.name,
				fmt.Sprintf("Read(): Failed to start command \"%s\": %v", command.String(), err),
			)
			continue
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
			continue
		}
	}

	// Read configured files
	for _, filename := range m.files {
		file, err := os.Open(filename)
		if err != nil {
			cclog.ComponentError(
				m.name,
				fmt.Sprintf("Read(): Failed to open file \"%s\": %v\n", filename, err),
			)
			continue
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
			continue
		}
	}
}

func (m *CustomCmdCollector) Close() {
	m.init = false
}
