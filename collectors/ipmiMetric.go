// Copyright (C) NHR@FAU, University Erlangen-Nuremberg.
// All rights reserved. This file is part of cc-lib.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.
// additional authors:
// Holger Obermaier (NHR@KIT)

package collectors

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"time"

	cclog "github.com/ClusterCockpit/cc-lib/v2/ccLogger"
	lp "github.com/ClusterCockpit/cc-lib/v2/ccMessage"
)

const IPMISENSORS_PATH = `ipmi-sensors`

type IpmiCollector struct {
	metricCollector

	config struct {
		ExcludeDevices  []string `json:"exclude_devices"`
		IpmitoolPath    string   `json:"ipmitool_path"`
		IpmisensorsPath string   `json:"ipmisensors_path"`
	}
	ipmitool    string
	ipmisensors string
}

func (m *IpmiCollector) Init(config json.RawMessage) error {
	// Check if already initialized
	if m.init {
		return nil
	}

	m.name = "IpmiCollector"
	if err := m.setup(); err != nil {
		return fmt.Errorf("%s Init(): setup() call failed: %w", m.name, err)
	}
	m.parallel = true
	m.meta = map[string]string{
		"source": m.name,
		"group":  "IPMI",
	}
	// default path to IPMI tools
	m.config.IpmitoolPath = "ipmitool"
	m.config.IpmisensorsPath = "ipmi-sensors"
	if len(config) > 0 {
		err := json.Unmarshal(config, &m.config)
		if err != nil {
			return err
		}
	}
	// Check if executables ipmitool or ipmisensors are found
	p, err := exec.LookPath(m.config.IpmitoolPath)
	if err == nil {
		command := exec.Command(p)
		err := command.Run()
		if err != nil {
			cclog.ComponentError(m.name, fmt.Sprintf("Failed to execute %s: %v", p, err.Error()))
			m.ipmitool = ""
		} else {
			m.ipmitool = p
		}
	}
	p, err = exec.LookPath(m.config.IpmisensorsPath)
	if err == nil {
		command := exec.Command(p)
		err := command.Run()
		if err != nil {
			cclog.ComponentError(m.name, fmt.Sprintf("Failed to execute %s: %v", p, err.Error()))
			m.ipmisensors = ""
		} else {
			m.ipmisensors = p
		}
	}
	if len(m.ipmitool) == 0 && len(m.ipmisensors) == 0 {
		return errors.New("no usable IPMI reader found")
	}

	m.init = true
	return nil
}

func (m *IpmiCollector) readIpmiTool(cmd string, output chan lp.CCMessage) {
	// Setup ipmitool command
	command := exec.Command(cmd, "sensor")
	stdout, _ := command.StdoutPipe()
	errBuf := new(bytes.Buffer)
	command.Stderr = errBuf

	// start command
	if err := command.Start(); err != nil {
		cclog.ComponentError(
			m.name,
			fmt.Sprintf("readIpmiTool(): Failed to start command \"%s\": %v", command.String(), err),
		)
		return
	}

	// Read command output
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		lv := strings.Split(scanner.Text(), "|")
		if len(lv) < 3 {
			continue
		}
		v, err := strconv.ParseFloat(strings.TrimSpace(lv[1]), 64)
		if err == nil {
			name := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(lv[0]), " ", "_"))
			unit := strings.TrimSpace(lv[2])
			switch unit {
			case "Volts":
				unit = "Volts"
			case "degrees C":
				unit = "degC"
			case "degrees F":
				unit = "degF"
			case "Watts":
				unit = "Watts"
			}

			y, err := lp.NewMessage(name, map[string]string{"type": "node"}, m.meta, map[string]any{"value": v}, time.Now())
			if err == nil {
				y.AddMeta("unit", unit)
				output <- y
			}
		}
	}

	// Wait for command end
	if err := command.Wait(); err != nil {
		errMsg, _ := io.ReadAll(errBuf)
		cclog.ComponentError(
			m.name,
			fmt.Sprintf("readIpmiTool(): Failed to wait for the end of command \"%s\": %v\n", command.String(), err),
		)
		cclog.ComponentError(m.name, fmt.Sprintf("readIpmiTool(): command stderr: \"%s\"\n", strings.TrimSpace(string(errMsg))))
		return
	}
}

func (m *IpmiCollector) readIpmiSensors(cmd string, output chan lp.CCMessage) {
	// Setup ipmisensors command
	command := exec.Command(cmd, "--comma-separated-output", "--sdr-cache-recreate")
	stdout, _ := command.StdoutPipe()
	errBuf := new(bytes.Buffer)
	command.Stderr = errBuf

	// start command
	if err := command.Start(); err != nil {
		cclog.ComponentError(
			m.name,
			fmt.Sprintf("readIpmiSensors(): Failed to start command \"%s\": %v", command.String(), err),
		)
		return
	}

	// Read command output
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		lv := strings.Split(scanner.Text(), ",")
		if len(lv) > 3 {
			v, err := strconv.ParseFloat(lv[3], 64)
			if err == nil {
				name := strings.ToLower(strings.ReplaceAll(lv[1], " ", "_"))
				y, err := lp.NewMessage(name, map[string]string{"type": "node"}, m.meta, map[string]any{"value": v}, time.Now())
				if err == nil {
					if len(lv) > 4 {
						y.AddMeta("unit", lv[4])
					}
					output <- y
				}
			}
		}
	}

	// Wait for command end
	if err := command.Wait(); err != nil {
		errMsg, _ := io.ReadAll(errBuf)
		cclog.ComponentError(
			m.name,
			fmt.Sprintf("readIpmiSensors(): Failed to wait for the end of command \"%s\": %v\n", command.String(), err),
		)
		cclog.ComponentError(m.name, fmt.Sprintf("readIpmiSensors(): command stderr: \"%s\"\n", strings.TrimSpace(string(errMsg))))
		return
	}
}

func (m *IpmiCollector) Read(interval time.Duration, output chan lp.CCMessage) {
	// Check if already initialized
	if !m.init {
		return
	}

	if len(m.config.IpmitoolPath) > 0 {
		m.readIpmiTool(m.config.IpmitoolPath, output)
	} else if len(m.config.IpmisensorsPath) > 0 {
		m.readIpmiSensors(m.config.IpmisensorsPath, output)
	}
}

func (m *IpmiCollector) Close() {
	m.init = false
}
