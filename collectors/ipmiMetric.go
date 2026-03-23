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
		Sudo            bool     `json:"use_sudo"`
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
		d := json.NewDecoder(bytes.NewReader(config))
		d.DisallowUnknownFields()
		if err := d.Decode(&m.config); err != nil {
			return fmt.Errorf("%s Init(): Error decoding JSON config: %w", m.name, err)
		}
	}

	if len(m.config.IpmitoolPath) != 0 && len(m.config.IpmisensorsPath) != 0 {
		return fmt.Errorf("ipmitool_path and ipmisensors_path cannot be used at the same time. Please disable one of them")
	}

	// Test if the configured commands actually work
	if len(m.config.IpmitoolPath) != 0 {
		dummyChan := make(chan lp.CCMessage)
		go func() {
			for range dummyChan {
			}
		}()
		err := m.readIpmiTool(dummyChan)
		close(dummyChan)
		if err != nil {
			return fmt.Errorf("Cannot execute '%s' (sudo=%t): %v", m.config.IpmitoolPath, m.config.Sudo, err)
		}
	} else if len(m.config.IpmisensorsPath) != 0 {
		dummyChan := make(chan lp.CCMessage)
		go func() {
			for range dummyChan {
			}
		}()
		err := m.readIpmiSensors(dummyChan)
		close(dummyChan)
		if err != nil {
			return fmt.Errorf("Cannot execute '%s' (sudo=%t): %v", m.config.IpmisensorsPath, m.config.Sudo, err)
		}
	} else {
		return fmt.Errorf("IpmiCollector enabled, but neither ipmitool nor ipmi-sensors are configured.")
	}

	m.init = true
	return nil
}

func (m *IpmiCollector) readIpmiTool(output chan lp.CCMessage) error {
	// Setup ipmitool command
	argv := make([]string, 0)
	if m.config.Sudo {
		argv = append(argv, "sudo")
	}
	argv = append(argv, m.config.IpmitoolPath, "sensor")
	command := exec.Command(argv[0], argv[1:]...)
	stdout, _ := command.StdoutPipe()
	errBuf := new(bytes.Buffer)
	command.Stderr = errBuf

	// start command
	if err := command.Start(); err != nil {
		return fmt.Errorf("Failed to start command '%s': %v", command.String(), err)
	}

	// Read command output
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		lv := strings.Split(scanner.Text(), "|")
		if len(lv) < 3 {
			continue
		}
		v, err := strconv.ParseFloat(strings.TrimSpace(lv[1]), 64)
		if err != nil {
			cclog.ComponentErrorf(m.name, "Failed to parse float '%s': %v", lv[1], err)
			continue
		}
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
		if err != nil {
			cclog.ComponentErrorf(m.name, "Failed to create message: %v", err)
			continue
		}
		y.AddMeta("unit", unit)
		output <- y
	}

	// Wait for command end
	if err := command.Wait(); err != nil {
		errMsg, _ := io.ReadAll(errBuf)
		return fmt.Errorf("Failed to complete command '%s': %v (stderr: %s)", command.String(), err, strings.TrimSpace(string(errMsg)))
	}

	return nil
}

func (m *IpmiCollector) readIpmiSensors(output chan lp.CCMessage) error {
	// Setup ipmisensors command
	argv := make([]string, 0)
	if m.config.Sudo {
		argv = append(argv, "sudo")
	}
	argv = append(argv, m.config.IpmisensorsPath, "--comma-separated-output", "--sdr-cache-recreate")
	command := exec.Command(argv[0], argv[1:]...)
	stdout, _ := command.StdoutPipe()
	errBuf := new(bytes.Buffer)
	command.Stderr = errBuf

	// start command
	if err := command.Start(); err != nil {
		return fmt.Errorf("Failed to start command '%s': %v", command.String(), err)
	}

	// Read command output
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		lv := strings.Split(scanner.Text(), ",")
		if len(lv) <= 3 {
			continue
		}
		v, err := strconv.ParseFloat(lv[3], 64)
		if err != nil {
			cclog.ComponentErrorf(m.name, "Failed to parse float '%s': %v", lv[3], err)
			continue
		}
		name := strings.ToLower(strings.ReplaceAll(lv[1], " ", "_"))
		y, err := lp.NewMessage(name, map[string]string{"type": "node"}, m.meta, map[string]any{"value": v}, time.Now())
		if err != nil {
			cclog.ComponentErrorf(m.name, "Failed to create message: %v", err)
			continue
		}
		if len(lv) > 4 {
			y.AddMeta("unit", lv[4])
		}
		output <- y
	}

	// Wait for command end
	if err := command.Wait(); err != nil {
		errMsg, _ := io.ReadAll(errBuf)
		return fmt.Errorf("Failed to complete command '%s': %v (stderr: %s)", command.String(), err, strings.TrimSpace(string(errMsg)))
	}

	return nil
}

func (m *IpmiCollector) Read(interval time.Duration, output chan lp.CCMessage) {
	// Check if already initialized
	if !m.init {
		return
	}

	if len(m.config.IpmitoolPath) > 0 {
		err := m.readIpmiTool(output)
		if err != nil {
			cclog.ComponentErrorf(m.name, "readIpmiTool() failed: %v", err)
		}
	} else if len(m.config.IpmisensorsPath) > 0 {
		err := m.readIpmiSensors(output)
		if err != nil {
			cclog.ComponentErrorf(m.name, "readIpmiSensors() failed: %v", err)
		}
	}
}

func (m *IpmiCollector) Close() {
	m.init = false
}
