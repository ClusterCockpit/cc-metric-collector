package collectors

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"strings"
	"time"

	cclog "github.com/ClusterCockpit/cc-metric-collector/internal/ccLogger"
	lp "github.com/ClusterCockpit/cc-metric-collector/internal/ccMetric"
)

const NETSTATFILE = "/proc/net/dev"

type NetstatCollectorConfig struct {
	IncludeDevices     []string `json:"include_devices"`
	SendAbsoluteValues bool     `json:"send_abs_values"`
	SendDerivedValues  bool     `json:"send_derived_values"`
}

type NetstatCollectorMetric struct {
	name      string
	index     int
	devtags   map[string]string
	lastValue float64
}

type NetstatCollector struct {
	metricCollector
	config        NetstatCollectorConfig
	matches       map[string]map[string]NetstatCollectorMetric
	lastTimestamp time.Time
}

func (m *NetstatCollector) Init(config json.RawMessage) error {
	m.name = "NetstatCollector"
	m.setup()
	m.lastTimestamp = time.Now()
	m.meta = map[string]string{
		"source": m.name,
		"group":  "Network",
	}

	const (
		fieldInterface          = iota
		fieldReceiveBytes       = iota
		fieldReceivePackets     = iota
		fieldReceiveErrs        = iota
		fieldReceiveDrop        = iota
		fieldReceiveFifo        = iota
		fieldReceiveFrame       = iota
		fieldReceiveCompressed  = iota
		fieldReceiveMulticast   = iota
		fieldTransmitBytes      = iota
		fieldTransmitPackets    = iota
		fieldTransmitErrs       = iota
		fieldTransmitDrop       = iota
		fieldTransmitFifo       = iota
		fieldTransmitColls      = iota
		fieldTransmitCarrier    = iota
		fieldTransmitCompressed = iota
	)

	nameIndexMap := map[string]int{
		"net_bytes_in":  fieldReceiveBytes,
		"net_pkts_in":   fieldReceivePackets,
		"net_bytes_out": fieldTransmitBytes,
		"net_pkts_out":  fieldTransmitPackets,
	}

	m.matches = make(map[string]map[string]NetstatCollectorMetric)

	// Set default configuration,
	m.config.SendAbsoluteValues = true
	m.config.SendDerivedValues = false
	// Read configuration file, allow overwriting default config
	if len(config) > 0 {
		err := json.Unmarshal(config, &m.config)
		if err != nil {
			cclog.ComponentError(m.name, "Error reading config:", err.Error())
			return err
		}
	}

	// Check access to net statistic file
	file, err := os.Open(NETSTATFILE)
	if err != nil {
		cclog.ComponentError(m.name, err.Error())
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		l := scanner.Text()

		// Skip lines with no net device entry
		if !strings.Contains(l, ":") {
			continue
		}

		// Split line into fields
		f := strings.Fields(l)

		// Get net device entry
		dev := strings.Trim(f[0], ": ")

		// Check if device is a included device
		if _, ok := stringArrayContains(m.config.IncludeDevices, dev); ok {
			m.matches[dev] = make(map[string]NetstatCollectorMetric)
			for name, idx := range nameIndexMap {
				m.matches[dev][name] = NetstatCollectorMetric{
					name:      name,
					index:     idx,
					lastValue: 0,
					devtags: map[string]string{
						"device": dev,
						"type":   "node",
					},
				}
			}
		}
	}
	if len(m.matches) == 0 {
		return errors.New("no devices to collector metrics found")
	}
	m.init = true
	return nil
}

func (m *NetstatCollector) Read(interval time.Duration, output chan lp.CCMetric) {
	if !m.init {
		return
	}
	// Current time stamp
	now := time.Now()
	// time difference to last time stamp
	timeDiff := now.Sub(m.lastTimestamp).Seconds()
	// Save current timestamp
	m.lastTimestamp = now

	file, err := os.Open(string(NETSTATFILE))
	if err != nil {
		cclog.ComponentError(m.name, err.Error())
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		l := scanner.Text()

		// Skip lines with no net device entry
		if !strings.Contains(l, ":") {
			continue
		}

		// Split line into fields
		f := strings.Fields(l)

		// Get net device entry
		dev := strings.Trim(f[0], ":")

		// Check if device is a included device
		if devmetrics, ok := m.matches[dev]; ok {
			for name, data := range devmetrics {
				v, err := strconv.ParseFloat(f[data.index], 64)
				if err == nil {
					if m.config.SendAbsoluteValues {
						if y, err := lp.New(name, data.devtags, m.meta, map[string]interface{}{"value": v}, now); err == nil {
							switch {
							case strings.Contains(name, "byte"):
								y.AddMeta("unit", "bytes")
							case strings.Contains(name, "pkt"):
								y.AddMeta("unit", "packets")
							}
							output <- y
						}
					}
					if m.config.SendDerivedValues {

						vdiff := v - data.lastValue
						value := vdiff / timeDiff
						if data.lastValue == 0 {
							value = 0
						}
						data.lastValue = v
						if y, err := lp.New(name+"_bw", data.devtags, m.meta, map[string]interface{}{"value": value}, now); err == nil {
							switch {
							case strings.Contains(name, "byte"):
								y.AddMeta("unit", "bytes/sec")
							case strings.Contains(name, "pkt"):
								y.AddMeta("unit", "packets/sec")
							}
							output <- y
						}
					}
					devmetrics[name] = data
				}
			}
		}
	}
}

func (m *NetstatCollector) Close() {
	m.init = false
}
