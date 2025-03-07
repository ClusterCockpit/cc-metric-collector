package collectors

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"strings"
	"time"

	lp "github.com/ClusterCockpit/cc-lib/ccMessage"
	cclog "github.com/ClusterCockpit/cc-metric-collector/pkg/ccLogger"
)

const IOSTATFILE = `/proc/diskstats`
const IOSTAT_SYSFSPATH = `/sys/block`

type IOstatCollectorConfig struct {
	ExcludeMetrics     []string `json:"exclude_metrics,omitempty"`
	OnlyMetrics        []string `json:"only_metrics,omitempty"`
	ExcludeDevices     []string `json:"exclude_devices,omitempty"`
	SendAbsoluteValues *bool    `json:"send_abs_values,omitempty"`
	SendDiffValues     *bool    `json:"send_diff_values,omitempty"`
	SendDerivedValues  *bool    `json:"send_derived_values,omitempty"`
}

// Helper methods for default values.
// - send_abs_values defaults to true,
// - send_diff_values and send_derived_values default to false.
func (cfg *IOstatCollectorConfig) AbsValues() bool {
	if cfg.SendAbsoluteValues == nil {
		return true
	}
	return *cfg.SendAbsoluteValues
}
func (cfg *IOstatCollectorConfig) DiffValues() bool {
	if cfg.SendDiffValues == nil {
		return false
	}
	return *cfg.SendDiffValues
}
func (cfg *IOstatCollectorConfig) DerivedValues() bool {
	if cfg.SendDerivedValues == nil {
		return false
	}
	return *cfg.SendDerivedValues
}

type IOstatCollectorEntry struct {
	lastValues map[string]int64
	tags       map[string]string
}

type IOstatCollector struct {
	metricCollector
	matches map[string]int
	config  IOstatCollectorConfig
	devices map[string]IOstatCollectorEntry
}

// shouldOutput returns true if a metric should be forwarded based on only_metrics and exclude_metrics.
func (m *IOstatCollector) shouldOutput(metricName string) bool {
	if len(m.config.OnlyMetrics) > 0 {
		for _, name := range m.config.OnlyMetrics {
			if name == metricName {
				return true
			}
		}
		return false
	}
	for _, name := range m.config.ExcludeMetrics {
		if name == metricName {
			return false
		}
	}
	return true
}

func (m *IOstatCollector) Init(config json.RawMessage) error {
	var err error
	m.name = "IOstatCollector"
	m.parallel = true
	m.meta = map[string]string{"source": m.name, "group": "Disk"}
	m.setup()
	if len(config) > 0 {
		err = json.Unmarshal(config, &m.config)
		if err != nil {
			return err
		}
	}
	// Define mapping from metric names to field indices in /proc/diskstats.
	allMatches := map[string]int{
		"io_reads":             3,
		"io_reads_merged":      4,
		"io_read_sectors":      5,
		"io_read_ms":           6,
		"io_writes":            7,
		"io_writes_merged":     8,
		"io_writes_sectors":    9,
		"io_writes_ms":         10,
		"io_ioops":             11,
		"io_ioops_ms":          12,
		"io_ioops_weighted_ms": 13,
		"io_discards":          14,
		"io_discards_merged":   15,
		"io_discards_sectors":  16,
		"io_discards_ms":       17,
		"io_flushes":           18,
		"io_flushes_ms":        19,
	}
	m.matches = make(map[string]int)
	// Allow a metric if either its base name, or base name+"_diff" or base name+"_rate" is present in only_metrics.
	for k, v := range allMatches {
		allowed := false
		if len(m.config.OnlyMetrics) > 0 {
			for _, metric := range m.config.OnlyMetrics {
				if metric == k || metric == k+"_diff" || metric == k+"_rate" {
					allowed = true
					break
				}
			}
		} else {
			if _, skip := stringArrayContains(m.config.ExcludeMetrics, k); !skip {
				allowed = true
			}
		}
		if allowed {
			m.matches[k] = v
		}
	}
	if len(m.matches) == 0 {
		return errors.New("no metrics to collect")
	}
	m.devices = make(map[string]IOstatCollectorEntry)
	file, err := os.Open(IOSTATFILE)
	if err != nil {
		cclog.ComponentError(m.name, err.Error())
		return err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		linefields := strings.Fields(line)
		if len(linefields) < 3 {
			continue
		}
		device := linefields[2]
		if strings.Contains(device, "loop") {
			continue
		}
		if _, skip := stringArrayContains(m.config.ExcludeDevices, device); skip {
			continue
		}
		values := make(map[string]int64)
		for mname, idx := range m.matches {
			if idx < len(linefields) {
				x, err := strconv.ParseInt(linefields[idx], 0, 64)
				if err == nil {
					values[mname] = x
				}
			} else {
				values[mname] = 0
			}
		}
		m.devices[device] = IOstatCollectorEntry{
			tags: map[string]string{
				"device": device,
				"type":   "node",
			},
			lastValues: values,
		}
	}
	m.init = true
	return err
}

func (m *IOstatCollector) Read(interval time.Duration, output chan lp.CCMessage) {
	if !m.init {
		return
	}
	file, err := os.Open(IOSTATFILE)
	if err != nil {
		cclog.ComponentError(m.name, err.Error())
		return
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) == 0 {
			continue
		}
		linefields := strings.Fields(line)
		if len(linefields) < 3 {
			continue
		}
		device := linefields[2]
		if strings.Contains(device, "loop") {
			continue
		}
		if _, skip := stringArrayContains(m.config.ExcludeDevices, device); skip {
			continue
		}
		entry, ok := m.devices[device]
		if !ok {
			continue
		}
		for name, idx := range m.matches {
			if idx >= len(linefields) {
				continue
			}
			x, err := strconv.ParseInt(linefields[idx], 0, 64)
			if err != nil {
				continue
			}
			// Send absolute metric if enabled.
			if m.config.AbsValues() && m.shouldOutput(name) {
				msg, err := lp.NewMessage(name, entry.tags, m.meta, map[string]interface{}{"value": int(x)}, time.Now())
				if err == nil {
					output <- msg
				}
			}
			diff := x - entry.lastValues[name]
			// Send diff metric if enabled.
			if m.config.DiffValues() && m.shouldOutput(name+"_diff") {
				msg, err := lp.NewMessage(name+"_diff", entry.tags, m.meta, map[string]interface{}{"value": int(diff)}, time.Now())
				if err == nil {
					output <- msg
				}
			}
			// Send derived metric if enabled.
			if m.config.DerivedValues() && m.shouldOutput(name+"_rate") {
				rate := float64(diff) / interval.Seconds()
				msg, err := lp.NewMessage(name+"_rate", entry.tags, m.meta, map[string]interface{}{"value": rate}, time.Now())
				if err == nil {
					output <- msg
				}
			}
			entry.lastValues[name] = x
		}
		m.devices[device] = entry
	}
}

func (m *IOstatCollector) Close() {
	m.init = false
}
