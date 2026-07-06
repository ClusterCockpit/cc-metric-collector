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

	m.name = "LenovoDensePowerCollector"
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
		return fmt.Errorf("energy reading test failed: %w", err)
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

	// The Lenovo hardware wants this magic byte string
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
		return 0, time.Unix(0, 0), fmt.Errorf("failed to run ipmitool: %w (stdout=%s stderr=%s)", err, stdout.String(), stderr.String())
	}

	data, err := hex.DecodeString(strings.ReplaceAll(strings.TrimSpace(stdout.String()), " ", ""))
	if err != nil {
		return 0, time.Unix(0, 0), fmt.Errorf("unable to decode ipmitool hex output: %w (stdout=%s)", err, stdout.String())
	}

	if len(data) != 14 {
		return 0, time.Unix(0, 0), fmt.Errorf("result must be 14 bytes as specified in the documentation")
	}

	// data is layed out like this:
	// data[0 to 1]: FIFO index of value (for debug only)
	// data[2 to 5]: Integer part of energy in J (little endian)
	// data[6 to 7]: Fraction part of energy in mJ (little endian)
	// data[8 to 11]: Integer part of Unix time of measurement in seconds (little endian)
	// data[12 to 13]: Fraction part of Unix time of measurement in milliseconds (little endian)
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
	if energyTime.Compare(m.energyTimeLast) != 0 {
		// Check for overflow
		energyValDiff := int64(energyVal) - int64(m.energyValLast)
		if energyVal < m.energyValLast {
			energyValDiff += int64(0x100000000)
		}
		energyTimeDiff := energyTime.Sub(m.energyTimeLast)
		if energyTime.Before(m.energyTimeLast) {
			energyTimeDiff = energyTimeDiff + 0x100000000*time.Second
		}
		powerVal = float64(energyValDiff) / energyTimeDiff.Seconds()

		m.energyValLast = energyVal
		m.energyTimeLast = energyTime
	}

	metric, err := lp.NewMetric("lenovo_node_power", map[string]string{"type": "node"}, m.meta, powerVal, energyTime)
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
