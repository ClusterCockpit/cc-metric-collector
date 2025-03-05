package collectors

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	lp "github.com/ClusterCockpit/cc-lib/ccMessage"
	cclog "github.com/ClusterCockpit/cc-metric-collector/pkg/ccLogger"
)

const NETSTATFILE = "/proc/net/dev"

type NetstatCollectorConfig struct {
	IncludeDevices     []string            `json:"include_devices"`
	SendAbsoluteValues bool                `json:"send_abs_values"`
	SendDerivedValues  bool                `json:"send_derived_values"`
	InterfaceAliases   map[string][]string `json:"interface_aliases,omitempty"`
	ExcludeMetrics     []string            `json:"exclude_metrics,omitempty"`
	OnlyMetrics        []string            `json:"only_metrics,omitempty"`
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
	config           NetstatCollectorConfig
	aliasToCanonical map[string]string
	matches          map[string][]NetstatCollectorMetric
	lastTimestamp    time.Time
}

func (m *NetstatCollector) buildAliasMapping() {
	m.aliasToCanonical = make(map[string]string)
	for canon, aliases := range m.config.InterfaceAliases {
		for _, alias := range aliases {
			m.aliasToCanonical[alias] = canon
		}
	}
}

func getCanonicalName(raw string, aliasToCanonical map[string]string) string {
	if canon, ok := aliasToCanonical[raw]; ok {
		return canon
	}
	return raw
}

func (m *NetstatCollector) shouldOutput(metricName string) bool {
	if len(m.config.OnlyMetrics) > 0 {
		for _, n := range m.config.OnlyMetrics {
			if n == metricName {
				return true
			}
		}
		return false
	}
	for _, n := range m.config.ExcludeMetrics {
		if n == metricName {
			return false
		}
	}
	return true
}

func (m *NetstatCollector) Init(config json.RawMessage) error {
	m.name = "NetstatCollector"
	m.parallel = true
	m.setup()
	m.lastTimestamp = time.Now()

	// Set default configuration
	m.config.SendAbsoluteValues = true
	m.config.SendDerivedValues = false

	if len(config) > 0 {
		if err := json.Unmarshal(config, &m.config); err != nil {
			cclog.ComponentError(m.name, "Error reading config:", err.Error())
			return err
		}
	}

	m.buildAliasMapping()

	// Open /proc/net/dev and read interfaces.
	file, err := os.Open(NETSTATFILE)
	if err != nil {
		cclog.ComponentError(m.name, err.Error())
		return err
	}
	defer file.Close()

	m.matches = make(map[string][]NetstatCollectorMetric)
	scanner := bufio.NewScanner(file)
	// Regex to split interface name and rest of line.
	reInterface := regexp.MustCompile(`^([^:]+):\s*(.*)$`)
	// Field positions based on /proc/net/dev format.
	const (
		fieldReceiveBytes = iota + 1
		fieldReceivePackets
		fieldTransmitBytes = 9
		fieldTransmitPackets
	)
	for scanner.Scan() {
		line := scanner.Text()
		// Skip header lines.
		if !strings.Contains(line, ":") {
			continue
		}
		matches := reInterface.FindStringSubmatch(line)
		if len(matches) != 3 {
			continue
		}
		raw := strings.TrimSpace(matches[1])
		canonical := getCanonicalName(raw, m.aliasToCanonical)
		// Check if device is included.
		if _, ok := stringArrayContains(m.config.IncludeDevices, canonical); ok {
			tags := map[string]string{"stype": "network", "stype-id": raw, "type": "node"}
			meta_unit_byte := map[string]string{"source": m.name, "group": "Network", "unit": "bytes"}
			meta_unit_byte_per_sec := map[string]string{"source": m.name, "group": "Network", "unit": "bytes/sec"}
			meta_unit_pkts := map[string]string{"source": m.name, "group": "Network", "unit": "packets"}
			meta_unit_pkts_per_sec := map[string]string{"source": m.name, "group": "Network", "unit": "packets/sec"}

			m.matches[canonical] = []NetstatCollectorMetric{
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
		return errors.New("no devices to collect metrics found")
	}
	m.init = true
	return nil
}

func (m *NetstatCollector) Read(interval time.Duration, output chan lp.CCMessage) {
	if !m.init {
		return
	}
	now := time.Now()
	timeDiff := now.Sub(m.lastTimestamp).Seconds()
	m.lastTimestamp = now

	file, err := os.Open(NETSTATFILE)
	if err != nil {
		cclog.ComponentError(m.name, err.Error())
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.Contains(line, ":") {
			continue
		}
		reInterface := regexp.MustCompile(`^([^:]+):\s*(.*)$`)
		matches := reInterface.FindStringSubmatch(line)
		if len(matches) != 3 {
			continue
		}
		raw := strings.TrimSpace(matches[1])
		canonical := getCanonicalName(raw, m.aliasToCanonical)
		if devmetrics, ok := m.matches[canonical]; ok {
			fields := strings.Fields(matches[2])
			for i := range devmetrics {
				metric := &devmetrics[i]
				v, err := strconv.ParseInt(fields[metric.index-1], 10, 64)
				if err != nil {
					continue
				}
				// Send absolute metric if enabled.
				if m.config.SendAbsoluteValues && m.shouldOutput(metric.name) {
					if y, err := lp.NewMessage(metric.name, metric.tags, metric.meta, map[string]interface{}{"value": v}, now); err == nil {
						output <- y
					}
				}
				// Send derived metric if enabled.
				if m.config.SendDerivedValues && metric.lastValue >= 0 && m.shouldOutput(metric.name+"_bw") {
					rate := float64(v-metric.lastValue) / timeDiff
					if y, err := lp.NewMessage(metric.name+"_bw", metric.tags, metric.meta_rates, map[string]interface{}{"value": rate}, now); err == nil {
						output <- y
					}
				}
				metric.lastValue = v
			}
		}
	}
}

func (m *NetstatCollector) Close() {
	m.init = false
}
