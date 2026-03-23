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

	m.ipmitool = m.config.IpmitoolPath
	m.ipmisensors = m.config.IpmisensorsPath

	// Test if any of the supported backends work
	var dummyChan chan lp.CCMessage
	dummyConsumer := func() {
		for range dummyChan {
		}
	}

	// Test if ipmi-sensors works (preferred over ipmitool, because it's faster)
	var ipmiSensorsErr error
	if _, ipmiSensorsErr = exec.LookPath(m.ipmisensors); ipmiSensorsErr == nil {
		dummyChan = make(chan lp.CCMessage)
		go dummyConsumer()
		ipmiSensorsErr = m.readIpmiSensors(dummyChan)
		close(dummyChan)
		if ipmiSensorsErr == nil {
			cclog.ComponentDebugf(m.name, "Using ipmi-sensors for ipmistat collector")
			m.init = true
			return nil
		}
	}
	cclog.ComponentDebugf(m.name, "Unable to use ipmi-sensors for ipmistat collector: %v", ipmiSensorsErr)
	m.ipmisensors = ""

	// Test if ipmitool works (may be very slow)
	var ipmiToolErr error
	if _, ipmiToolErr = exec.LookPath(m.ipmitool); ipmiToolErr == nil {
		dummyChan = make(chan lp.CCMessage)
		go dummyConsumer()
		ipmiToolErr = m.readIpmiTool(dummyChan)
		close(dummyChan)
		if ipmiToolErr == nil {
			cclog.ComponentDebugf(m.name, "Using ipmitool for ipmistat collector")
			m.init = true
			return nil
		}
	}
	m.ipmitool = ""
	cclog.ComponentDebugf(m.name, "Unable to use ipmitool for ipmistat collector: %v", ipmiToolErr)

	return fmt.Errorf("unable to init neither ipmitool (%w) nor ipmi-sensors (%w)", ipmiToolErr, ipmiSensorsErr)
}

func (m *IpmiCollector) readIpmiTool(output chan lp.CCMessage) error {
	// Setup ipmitool command
	argv := make([]string, 0)
	if m.config.Sudo {
		argv = append(argv, "sudo", "-n")
	}
	argv = append(argv, m.ipmitool, "sensor")
	command := exec.Command(argv[0], argv[1:]...)
	stdout, _ := command.StdoutPipe()
	errBuf := new(bytes.Buffer)
	command.Stderr = errBuf

	// start command
	if err := command.Start(); err != nil {
		return fmt.Errorf("failed to start command '%s': %w", command.String(), err)
	}

	// Read command output
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		lv := strings.Split(scanner.Text(), "|")
		if len(lv) < 3 {
			continue
		}

		if strings.TrimSpace(lv[1]) == "0x0" || strings.TrimSpace(lv[1]) == "na" {
			// Ignore known non-float values
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
		return fmt.Errorf("failed to complete command '%s': %w (stderr: %s)", command.String(), err, strings.TrimSpace(string(errMsg)))
	}

	return nil
}

func (m *IpmiCollector) readIpmiSensors(output chan lp.CCMessage) error {
	// Setup ipmisensors command
	argv := make([]string, 0)
	if m.config.Sudo {
		argv = append(argv, "sudo", "-n")
	}
	argv = append(argv, m.ipmisensors, "--comma-separated-output", "--sdr-cache-recreate")
	command := exec.Command(argv[0], argv[1:]...)
	stdout, _ := command.StdoutPipe()
	errBuf := new(bytes.Buffer)
	command.Stderr = errBuf

	// start command
	if err := command.Start(); err != nil {
		return fmt.Errorf("failed to start command '%s': %w", command.String(), err)
	}

	// Read command output
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		lv := strings.Split(scanner.Text(), ",")
		if len(lv) <= 3 {
			continue
		}
		if lv[3] == "N/A" || lv[3] == "Reading" {
			// Ignore known non-float values
			continue
		}
		v, err := strconv.ParseFloat(strings.TrimSpace(lv[3]), 64)
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
		return fmt.Errorf("failed to complete command '%s': %w (stderr: %s)", command.String(), err, strings.TrimSpace(string(errMsg)))
	}

	return nil
}

func (m *IpmiCollector) Read(interval time.Duration, output chan lp.CCMessage) {
	// Check if already initialized
	if !m.init {
		return
	}

	if len(m.ipmisensors) > 0 {
		err := m.readIpmiSensors(output)
		if err != nil {
			cclog.ComponentErrorf(m.name, "readIpmiSensors() failed: %v", err)
		}
	} else if len(m.ipmitool) > 0 {
		err := m.readIpmiTool(output)
		if err != nil {
			cclog.ComponentErrorf(m.name, "readIpmiTool() failed: %v", err)
		}
	}
}

func (m *IpmiCollector) Close() {
	m.init = false
}
