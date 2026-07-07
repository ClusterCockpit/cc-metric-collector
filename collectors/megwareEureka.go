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
	Cnt         int64   `json:"Cnt"` // No idea what this is, ignore
	Energy      float64 `json:"Energy"`
	Vin         float64 `json:"Vin"`
	Iin         float64 `json:"Iin"`
	Pin         float64 `json:"Pin"`
	PinAvg      float64 `json:"PinAvg"`
	Vout        float64 `json:"Vout"`
	Iout        float64 `json:"Iout"`
	Pout        float64 `json:"Pout"`
	StandbyVout float64 `json:"StandbyVout"`
	StandbyIout float64 `json:"StandbyIout"`
	StandbyPout float64 `json:"StandbyPout"`
	TempBusbar  float64 `json:"TempBusbar"`
	TempSsd     float64 `json:"TempSsd"`
	TempMps     float64 `json:"TempMps"`
	// No idea what the ones below mean exactl. Ignore them for now.
	EnergyTime         int64 `json:"EnergyTime"`
	EnergyAccumulator  int64 `json:"EnergyAccumulator"`
	EnergyRolloverCnt  int64 `json:"EnergyRolloverCnt"`
	EnergySampleCntU24 int64 `json:"EnergySampleCntU24"`

	Timestamp time.Time
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
		return fmt.Errorf("energy reading test failed: %w", err)
	}

	m.energyValLast = data.Energy
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
		return nil, fmt.Errorf("failed to run u20: %w (stdout=%s stderr=%s)", err, stdout.String(), stderr.String())
	}

	var u20output struct {
		GetMpsPollValues mpsData `json:"GET_MPS_POLL_VALUES"`
	}

	err = json.Unmarshal(stdout.Bytes(), &u20output)
	if err != nil {
		return nil, fmt.Errorf("unable to decode u20 JSON output: %w (stdout=%s)", err, stdout.String())
	}

	u20output.GetMpsPollValues.Timestamp = time.Now()

	return &u20output.GetMpsPollValues, nil
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

	if data.Timestamp.After(m.energyTimeLast) {
		// Important, m.energy comes in Wh, so multiply by 3600 to get Ws (aka Joule)
		energyValDiff := data.Energy - m.energyValLast
		energyTimeDiff := data.Timestamp.Sub(m.energyTimeLast)
		powerVal = energyValDiff * 3600 / energyTimeDiff.Seconds()

		m.energyValLast = data.Energy
		m.energyTimeLast = data.Timestamp
	}

	metricNamePrefix := "eureka_"
	metricMap := map[string]struct {
		value any
		unit  string
	}{
		"power":        {value: powerVal, unit: "Watts"},
		"vin":          {value: data.Vin, unit: "Volts"},
		"iin":          {value: data.Iin, unit: "Amperes"},
		"pin":          {value: data.Pin, unit: "Watts"},
		"pin_avg":      {value: data.PinAvg, unit: "Watts"},
		"vout":         {value: data.Vout, unit: "Volts"},
		"iout":         {value: data.Iout, unit: "Amperes"},
		"pout":         {value: data.Pout, unit: "Watts"},
		"standby_vout": {value: data.StandbyVout, unit: "Volts"},
		"standby_iout": {value: data.StandbyIout, unit: "Amperes"},
		"standby_pout": {value: data.StandbyPout, unit: "Watts"},
		"temp_busbar":  {value: data.TempBusbar, unit: "degC"},
		"temp_ssd":     {value: data.TempSsd, unit: "degC"},
		"temp_mps":     {value: data.TempMps, unit: "degC"},
	}

	for metricName, metricData := range metricMap {
		metricName = metricNamePrefix + metricName
		metric, err := lp.NewMetric(metricName, map[string]string{"type": "node"}, m.meta, metricData.value, data.Timestamp)
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
