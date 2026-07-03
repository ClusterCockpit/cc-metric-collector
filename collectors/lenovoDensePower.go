// Copyright (C) NHR@FAU, University Erlangen-Nuremberg.
// All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.
// additional authors:
// Michael Panzlaff (NHR@FAU)

package collectors

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	cclog "github.com/ClusterCockpit/cc-lib/v2/ccLogger"
	lp "github.com/ClusterCockpit/cc-lib/v2/ccMessage"
)

type LenovoDensePowerCollector struct {
	metricCollector

	config struct {
		IpmitoolPath string `json:"ipmitool_path"`
		Sudo         bool   `json:"use_sudo"`
	}

	ipmitool string

	energyValLast  float64
	energyTimeLast time.Time
}

func (m *LenovoDensePowerCollector) Init(config json.RawMessage) error {
	// Check if already initialized
	if m.init {
		return nil
	}

	m.name = "LenovoDensePower"
	if err := m.setup(); err != nil {
		return fmt.Errorf("%s Init(): setup() call failed: %w", m.name, err)
	}
	m.meta = map[string]string{
		"source": m.name,
		"group":  "IPMI",
	}

	m.config.IpmitoolPath = "ipmitool"

	if len(config) > 0 {
		d := json.NewDecoder(bytes.NewReader(config))
		d.DisallowUnknownFields()
		if err := d.Decode(&m.config); err != nil {
			return fmt.Errorf("%s Init(): Error decoding JSON config: %w", m.name, err)
		}
	}

	m.ipmitool = m.config.IpmitoolPath

	energyVal, energyTime, err := m.readEnergy()
	if err != nil {
		return fmt.Errorf("Energy reading test failed: %w", err)
	}

	m.energyValLast = energyVal
	m.energyTimeLast = energyTime
	m.init = true

	return nil
}

func (m *LenovoDensePowerCollector) readEnergy() (float64, time.Time, error) {
	argv := make([]string, 0)
	if m.config.Sudo {
		argv = append(argv, "sudo", "-n")
	}

	lenovoRequestEnergyMsg := []uint8{0x3a, 0x32, 4, 2, 0, 0, 0}

	argv = append(argv, m.ipmitool, "raw")

	for _, val := range lenovoRequestEnergyMsg {
		argv = append(argv, strconv.Itoa(int(val)))
	}

	cmd := exec.Command(argv[0], argv[1:]...)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return 0, time.Unix(0, 0), fmt.Errorf("Failed to run ipmitool: %w (stdout=%s stderr=%s)", err, stdout.String(), stderr.String())
	}

	data, err := hex.DecodeString(strings.ReplaceAll(strings.TrimSpace(stdout.String()), " ", ""))
	if err != nil {
		return 0, time.Unix(0, 0), fmt.Errorf("Unable to decode ipmitool hex output: %w (stdout=%s)", err, stdout.String())
	}

	if len(data) != 14 {
		return 0, time.Unix(0, 0), fmt.Errorf("Result must be 14 bytes as specified in the documentation")
	}

	wholeJoules := (uint32(data[2]) << 0) | (uint32(data[3]) << 8) | (uint32(data[4]) << 16) | (uint32(data[5]) << 24)
	milliJoules := uint16(data[6]) | (uint16(data[7]) << 8)
	finalJoules := float64(wholeJoules) + float64(milliJoules)*1e-3

	wholeSeconds := (uint32(data[8]) << 0) | (uint32(data[9]) << 8) | (uint32(data[10]) << 16) | (uint32(data[11]) << 24)
	milliSeconds := uint16(data[12]) | (uint16(data[13]) << 8)
	finalTime := time.Unix(int64(wholeSeconds), (int64(milliSeconds) * 1000000))

	return finalJoules, finalTime, nil
}

func (m *LenovoDensePowerCollector) Read(interval time.Duration, output chan lp.CCMessage) {
	// Check if already initialized
	if !m.init {
		return
	}

	energyVal, energyTime, err := m.readEnergy()
	if err != nil {
		cclog.ComponentErrorf(m.name, "readEnergy failed: %v", err)
		return
	}

	powerVal := 0.0
	if energyTime.After(m.energyTimeLast) {
		energyValDiff := energyVal - m.energyValLast
		energyTimeDiff := energyTime.Sub(m.energyTimeLast)
		powerVal = energyValDiff / energyTimeDiff.Seconds()

		m.energyValLast = energyVal
		m.energyTimeLast = energyTime
	}

	metric, err := lp.NewMetric("node_power", map[string]string{"type": "node"}, m.meta, powerVal, energyTime)
	if err != nil {
		cclog.ComponentErrorf(m.name, "lp.NewMetric failed: %v", err)
		return
	}
	metric.AddMeta("unit", "Watts")
	output <- metric
}

func (m *LenovoDensePowerCollector) Close() {
	m.init = false
}
