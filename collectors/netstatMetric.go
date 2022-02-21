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

const NETSTATFILE = `/proc/net/dev`

type NetstatCollectorConfig struct {
	IncludeDevices []string `json:"include_devices"`
}

type NetstatCollectorMetric struct {
	index     int
	lastValue float64
}

type NetstatCollector struct {
	metricCollector
	config        NetstatCollectorConfig
	matches       map[string]map[string]NetstatCollectorMetric
	devtags       map[string]map[string]string
	lastTimestamp time.Time
}

func (m *NetstatCollector) Init(config json.RawMessage) error {
	m.name = "NetstatCollector"
	m.setup()
	m.lastTimestamp = time.Now()
	m.meta = map[string]string{"source": m.name, "group": "Network"}
	m.devtags = make(map[string]map[string]string)
	nameIndexMap := map[string]int{
		"net_bytes_in":  1,
		"net_pkts_in":   2,
		"net_bytes_out": 9,
		"net_pkts_out":  10,
	}
	m.matches = make(map[string]map[string]NetstatCollectorMetric)
	if len(config) > 0 {
		err := json.Unmarshal(config, &m.config)
		if err != nil {
			cclog.ComponentError(m.name, "Error reading config:", err.Error())
			return err
		}
	}
	file, err := os.Open(string(NETSTATFILE))
	if err != nil {
		cclog.ComponentError(m.name, err.Error())
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		l := scanner.Text()
		if !strings.Contains(l, ":") {
			continue
		}
		f := strings.Fields(l)
		dev := strings.Trim(f[0], ": ")
		if _, ok := stringArrayContains(m.config.IncludeDevices, dev); ok {
			m.matches[dev] = make(map[string]NetstatCollectorMetric)
			for name, idx := range nameIndexMap {
				m.matches[dev][name] = NetstatCollectorMetric{
					index:     idx,
					lastValue: 0,
				}
			}
			m.devtags[dev] = map[string]string{"device": dev, "type": "node"}
		}
	}
	if len(m.devtags) == 0 {
		return errors.New("no devices to collector metrics found")
	}
	m.init = true
	return nil
}

func (m *NetstatCollector) Read(interval time.Duration, output chan lp.CCMetric) {
	if !m.init {
		return
	}
	now := time.Now()
	file, err := os.Open(string(NETSTATFILE))
	if err != nil {
		cclog.ComponentError(m.name, err.Error())
		return
	}
	defer file.Close()
	tdiff := now.Sub(m.lastTimestamp)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		l := scanner.Text()
		if !strings.Contains(l, ":") {
			continue
		}
		f := strings.Fields(l)
		dev := strings.Trim(f[0], ":")

		if devmetrics, ok := m.matches[dev]; ok {
			for name, data := range devmetrics {
				v, err := strconv.ParseFloat(f[data.index], 64)
				if err == nil {
					vdiff := v - data.lastValue
					value := vdiff / tdiff.Seconds()
					if data.lastValue == 0 {
						value = 0
					}
					data.lastValue = v
					y, err := lp.New(name, m.devtags[dev], m.meta, map[string]interface{}{"value": value}, now)
					if err == nil {
						switch {
						case strings.Contains(name, "byte"):
							y.AddMeta("unit", "bytes/sec")
						case strings.Contains(name, "pkt"):
							y.AddMeta("unit", "packets/sec")
						}
						output <- y
					}
					devmetrics[name] = data
				}
			}
		}
	}
	m.lastTimestamp = time.Now()
}

func (m *NetstatCollector) Close() {
	m.init = false
}
