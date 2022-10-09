package collectors

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"strings"
	"time"

	cclog "github.com/ClusterCockpit/cc-metric-collector/pkg/ccLogger"
	lp "github.com/ClusterCockpit/cc-metric-collector/pkg/ccMetric"
)

const NETSTATFILE = "/proc/net/dev"

type NetstatCollectorConfig struct {
	IncludeDevices     []string `json:"include_devices"`
	SendAbsoluteValues bool     `json:"send_abs_values"`
	SendDerivedValues  bool     `json:"send_derived_values"`
}

type NetstatCollectorMetric struct {
	name       string
	index      int
	tags       map[string]string
	meta       map[string]string
	meta_rates map[string]string
	lastValue  int64
}

type NetstatCollector struct {
	metricCollector
	config        NetstatCollectorConfig
	matches       map[string][]NetstatCollectorMetric
	lastTimestamp time.Time
}

func (m *NetstatCollector) Init(config json.RawMessage) error {
	m.name = "NetstatCollector"
	m.parallel = true
	m.setup()
	m.lastTimestamp = time.Now()

	const (
		fieldInterface = iota
		fieldReceiveBytes
		fieldReceivePackets
		fieldReceiveErrs
		fieldReceiveDrop
		fieldReceiveFifo
		fieldReceiveFrame
		fieldReceiveCompressed
		fieldReceiveMulticast
		fieldTransmitBytes
		fieldTransmitPackets
		fieldTransmitErrs
		fieldTransmitDrop
		fieldTransmitFifo
		fieldTransmitColls
		fieldTransmitCarrier
		fieldTransmitCompressed
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
			tags := map[string]string{"device": dev, "type": "node"}
			meta_unit_byte := map[string]string{"source": m.name, "group": "Network", "unit": "bytes"}
			meta_unit_byte_per_sec := map[string]string{"source": m.name, "group": "Network", "unit": "bytes/sec"}
			meta_unit_pkts := map[string]string{"source": m.name, "group": "Network", "unit": "packets"}
			meta_unit_pkts_per_sec := map[string]string{"source": m.name, "group": "Network", "unit": "packets/sec"}

			m.matches[dev] = []NetstatCollectorMetric{
				{
					name:       "net_bytes_in",
					index:      fieldReceiveBytes,
					lastValue:  -1,
					tags:       tags,
					meta:       meta_unit_byte,
					meta_rates: meta_unit_byte_per_sec,
				},
				{
					name:       "net_pkts_in",
					index:      fieldReceivePackets,
					lastValue:  -1,
					tags:       tags,
					meta:       meta_unit_pkts,
					meta_rates: meta_unit_pkts_per_sec,
				},
				{
					name:       "net_bytes_out",
					index:      fieldTransmitBytes,
					lastValue:  -1,
					tags:       tags,
					meta:       meta_unit_byte,
					meta_rates: meta_unit_byte_per_sec,
				},
				{
					name:       "net_pkts_out",
					index:      fieldTransmitPackets,
					lastValue:  -1,
					tags:       tags,
					meta:       meta_unit_pkts,
					meta_rates: meta_unit_pkts_per_sec,
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
				metric := &devmetrics[i]

				// Read value
				v, err := strconv.ParseInt(f[metric.index], 10, 64)
				if err != nil {
					continue
				}
				if m.config.SendAbsoluteValues {
					if y, err := lp.New(metric.name, metric.tags, metric.meta, map[string]interface{}{"value": v}, now); err == nil {
						output <- y
					}
				}
				if m.config.SendDerivedValues {
					if metric.lastValue >= 0 {
						rate := float64(v-metric.lastValue) / timeDiff
						if y, err := lp.New(metric.name+"_bw", metric.tags, metric.meta_rates, map[string]interface{}{"value": rate}, now); err == nil {
							output <- y
						}
					}
					metric.lastValue = v
				}
			}
		}
	}
}

func (m *NetstatCollector) Close() {
	m.init = false
}
