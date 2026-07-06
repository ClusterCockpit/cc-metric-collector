// Copyright (C) NHR@FAU, University Erlangen-Nuremberg.
// All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.
// additional authors:
// Michael Panzlaff (NHR@FAU)

package collectors

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"

	cclog "github.com/ClusterCockpit/cc-lib/v2/ccLogger"
	lp "github.com/ClusterCockpit/cc-lib/v2/ccMessage"
)

// MPS refers to Monolithic Power Systems, from which the MP5922 is used
// to measure telemetry on the Eureka platform.
type mpsData struct {
	cnt         int64   `json:"Cnt"` // No idea what this is, ignore
	energy      float64 `json:"Energy"`
	vin         float64 `json:"Vin"`
	iin         float64 `json:"Iin"`
	pin         float64 `json:"Pin"`
	pinAvg      float64 `json:"PinAvg"`
	vout        float64 `json:"Vout"`
	iout        float64 `json:"Iout"`
	pout        float64 `json:"Pout"`
	standbyVout float64 `json:"StandbyVout"`
	standbyIout float64 `json:"StandbyIout"`
	standbyPout float64 `json:"StandbyPout"`
	tempBusbar  float64 `json:"TempBusbar"`
	tempSsd     float64 `json:"TempSsd"`
	tempMps     float64 `json:"TempMps"`
	// No idea what the ones below mean exactl. Ignore them for now.
	energyTime         int64 `json:"EnergyTime"`
	energyAccumulator  int64 `json:"EnergyAccumulator"`
	energyRolloverCnt  int64 `json:"EnergyRolloverCnt"`
	energySampleCntU24 int64 `json:"EnergySampleCntU24"`

	timestamp time.Time
}

type MegwareEurekaCollector struct {
	metricCollector

	config struct {
		U20Path string `json:"u20_path"`
		Sudo    bool   `json:"use_sudo"`
	}

	u20path string

	energyValLast  float64
	energyTimeLast time.Time
}

func (m *MegwareEurekaCollector) Init(config json.RawMessage) error {
	// Check if already initialized
	if m.init {
		return nil
	}

	m.name = "MegwareEureka"
	if err := m.setup(); err != nil {
		return fmt.Errorf("%s Init(): setup() call failed: %w", m.name, err)
	}
	m.meta = map[string]string{
		"source": m.name,
		"group":  "U20",
	}

	m.config.U20Path = "u20"

	if len(config) > 0 {
		d := json.NewDecoder(bytes.NewReader(config))
		d.DisallowUnknownFields()
		if err := d.Decode(&m.config); err != nil {
			return fmt.Errorf("%s Init(): Error decoding JSON config: %w", m.name, err)
		}
	}

	m.u20path = m.config.U20Path

	data, err := m.readMpsData()
	if err != nil {
		return fmt.Errorf("Energy reading test failed: %w", err)
	}

	m.energyValLast = data.energy
	m.energyTimeLast = time.Now()
	m.init = true

	return nil
}

func (m *MegwareEurekaCollector) readMpsData() (*mpsData, error) {
	argv := make([]string, 0)
	if m.config.Sudo {
		argv = append(argv, "sudo", "-n")
	}

	argv = append(argv, m.u20path, "values", "--read", "GET_MPS_POLL_VALUES")

	cmd := exec.Command(argv[0], argv[1:]...)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("Failed to run u20: %w (stdout=%s stderr=%s)", err, stdout.String(), stderr.String())
	}

	var u20output struct {
		getMpsPollValues mpsData `json:"GET_MPS_POLL_VALUES"`
	}

	err = json.Unmarshal([]byte(stdout.String()), &u20output)
	if err != nil {
		return nil, fmt.Errorf("Unable to decode u20 JSON output: %w (stdout=%s)", err, stdout.String())
	}

	u20output.getMpsPollValues.timestamp = time.Now()

	return &u20output.getMpsPollValues, nil
}

func (m *MegwareEurekaCollector) Read(interval time.Duration, output chan lp.CCMessage) {
	// Check if already initialized
	if !m.init {
		return
	}

	data, err := m.readMpsData()
	if err != nil {
		cclog.ComponentErrorf(m.name, "readMpsData failed: %v", err)
		return
	}

	powerVal := 0.0

	if data.timestamp.After(m.energyTimeLast) {
		// Important, m.energy comes in Wh, so multiply by 3600 to get Ws (aka Joule)
		energyValDiff := data.energy - m.energyValLast
		energyTimeDiff := data.timestamp.Sub(m.energyTimeLast)
		powerVal = energyValDiff * 3600 / energyTimeDiff.Seconds()

		m.energyValLast = data.energy
		m.energyTimeLast = data.timestamp
	}

	metricNamePrefix := "eureka_"
	metricMap := map[string]struct {
		value any
		unit  string
	}{
		"power":        {value: powerVal, unit: "Watts"},
		"vin":          {value: data.vin, unit: "Volts"},
		"iin":          {value: data.iin, unit: "Amperes"},
		"pin":          {value: data.pin, unit: "Watts"},
		"pin_avg":      {value: data.pinAvg, unit: "Watts"},
		"vout":         {value: data.vout, unit: "Volts"},
		"iout":         {value: data.iout, unit: "Amperes"},
		"pout":         {value: data.pout, unit: "Watts"},
		"standby_vout": {value: data.standbyVout, unit: "Volts"},
		"standby_iout": {value: data.standbyIout, unit: "Amperes"},
		"standby_pout": {value: data.standbyPout, unit: "Watts"},
		"temp_busbar":  {value: data.tempBusbar, unit: "degC"},
		"temp_ssd":     {value: data.tempSsd, unit: "degC"},
		"temp_mps":     {value: data.tempMps, unit: "degC"},
	}

	for metricName, metricData := range metricMap {
		metricName = metricNamePrefix + metricName
		metric, err := lp.NewMetric(metricName, map[string]string{"type": "node"}, m.meta, metricData.value, data.timestamp)
		if err != nil {
			cclog.ComponentErrorf(m.name, "lp.NewMetric failed: %v", err)
			return
		}
		metric.AddMeta("unit", metricData.unit)
		output <- metric
	}
}

func (m *MegwareEurekaCollector) Close() {
	m.init = false
}
