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
	tags      map[string]string
	rate_tags map[string]string
	lastValue float64
}

type NetstatCollector struct {
	metricCollector
	config        NetstatCollectorConfig
	matches       map[string][]NetstatCollectorMetric
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

	m.matches = make(map[string][]NetstatCollectorMetric)

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
			m.matches[dev] = []NetstatCollectorMetric{
				{
					name:      "net_bytes_in",
					index:     fieldReceiveBytes,
					lastValue: 0,
					tags:      map[string]string{"device": dev, "type": "node", "unit": "bytes"},
					rate_tags: map[string]string{"device": dev, "type": "node", "unit": "bytes/sec"},
				},
				{
					name:      "net_pkts_in",
					index:     fieldReceivePackets,
					lastValue: 0,
					tags:      map[string]string{"device": dev, "type": "node", "unit": "packets"},
					rate_tags: map[string]string{"device": dev, "type": "node", "unit": "packets/sec"},
				},
				{
					name:      "net_bytes_out",
					index:     fieldTransmitBytes,
					lastValue: 0,
					tags:      map[string]string{"device": dev, "type": "node", "unit": "bytes"},
					rate_tags: map[string]string{"device": dev, "type": "node", "unit": "bytes/sec"},
				},
				{
					name:      "net_pkts_out",
					index:     fieldTransmitPackets,
					lastValue: 0,
					tags:      map[string]string{"device": dev, "type": "node", "unit": "packets"},
					rate_tags: map[string]string{"device": dev, "type": "node", "unit": "packets/sec"},
				},
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
			for i := range devmetrics {
				data := &devmetrics[i]

				// Read value
				v, err := strconv.ParseFloat(f[data.index], 64)
				if err != nil {
					continue
				}
				if m.config.SendAbsoluteValues {
					if y, err := lp.New(data.name, data.tags, m.meta, map[string]interface{}{"value": v}, now); err == nil {
						output <- y
					}
				}
				if m.config.SendDerivedValues {
					rate := (v - data.lastValue) / timeDiff
					if data.lastValue == 0 {
						rate = 0
					}
					data.lastValue = v
					if y, err := lp.New(data.name+"_bw", data.rate_tags, m.meta, map[string]interface{}{"value": rate}, now); err == nil {
						output <- y
					}
				}
			}
		}
	}
}

func (m *NetstatCollector) Close() {
	m.init = false
}
