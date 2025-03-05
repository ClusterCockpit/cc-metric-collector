package collectors

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	lp "github.com/ClusterCockpit/cc-lib/ccMessage"
	cclog "github.com/ClusterCockpit/cc-metric-collector/pkg/ccLogger"
)

// See: https://www.kernel.org/doc/html/latest/hwmon/sysfs-interface.html
// /sys/class/hwmon/hwmon*/name -> coretemp
// /sys/class/hwmon/hwmon*/temp*_label -> Core 0
// /sys/class/hwmon/hwmon*/temp*_input -> 27800 = 27.8°C
// /sys/class/hwmon/hwmon*/temp*_max -> 86000 = 86.0°C
// /sys/class/hwmon/hwmon*/temp*_crit -> 100000 = 100.0°C

type TempCollectorSensor struct {
	name         string
	label        string
	metricName   string // Default: name_label, in lowercase with underscores
	file         string
	maxTempName  string
	maxTemp      int64
	critTempName string
	critTemp     int64
	tags         map[string]string
}

type TempCollectorConfig struct {
	ExcludeMetrics     []string                     `json:"exclude_metrics,omitempty"`
	OnlyMetrics        []string                     `json:"only_metrics,omitempty"`
	TagOverride        map[string]map[string]string `json:"tag_override,omitempty"`
	ReportMaxTemp      bool                         `json:"report_max_temperature"`
	ReportCriticalTemp bool                         `json:"report_critical_temperature"`
}

type TempCollector struct {
	metricCollector
	config  TempCollectorConfig
	sensors []*TempCollectorSensor
}

// shouldOutput returns true if the metric should be sent.
// If OnlyMetrics is set, only metrics in that list are output.
// Otherwise, metrics in ExcludeMetrics are skipped.
func (m *TempCollector) shouldOutput(metricName string) bool {
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

func (m *TempCollector) Init(config json.RawMessage) error {
	if m.init {
		return nil
	}

	m.name = "TempCollector"
	m.parallel = true
	m.setup()
	if len(config) > 0 {
		if err := json.Unmarshal(config, &m.config); err != nil {
			return err
		}
	}

	m.meta = map[string]string{
		"source": m.name,
		"group":  "IPMI",
		"unit":   "degC",
	}

	m.sensors = make([]*TempCollectorSensor, 0)

	// Find all temperature sensor files
	globPattern := filepath.Join("/sys/class/hwmon", "*", "temp*_input")
	inputFiles, err := filepath.Glob(globPattern)
	if err != nil {
		return fmt.Errorf("unable to glob files with pattern '%s': %v", globPattern, err)
	}
	if inputFiles == nil {
		return fmt.Errorf("unable to find any files with pattern '%s'", globPattern)
	}

	// Get sensor name for each temperature sensor file
	for _, file := range inputFiles {
		sensor := new(TempCollectorSensor)

		// Read sensor name from the "name" file
		nameFile := filepath.Join(filepath.Dir(file), "name")
		if data, err := os.ReadFile(nameFile); err == nil {
			sensor.name = strings.TrimSpace(string(data))
		}

		// Read sensor label from the corresponding "_label" file
		labelFile := strings.TrimSuffix(file, "_input") + "_label"
		if data, err := os.ReadFile(labelFile); err == nil {
			sensor.label = strings.TrimSpace(string(data))
		}

		// Determine sensor metric name
		switch {
		case len(sensor.name) == 0 && len(sensor.label) == 0:
			continue
		case sensor.name == "coretemp" && (strings.HasPrefix(sensor.label, "Core ") || strings.HasPrefix(sensor.label, "Package id ")):
			sensor.metricName = "temp_" + sensor.label
		case len(sensor.name) != 0 && len(sensor.label) != 0:
			sensor.metricName = sensor.name + "_" + sensor.label
		case len(sensor.name) != 0:
			sensor.metricName = sensor.name
		case len(sensor.label) != 0:
			sensor.metricName = sensor.label
		}
		sensor.metricName = strings.ToLower(sensor.metricName)
		sensor.metricName = strings.Replace(sensor.metricName, " ", "_", -1)
		if !strings.Contains(sensor.metricName, "temp") {
			sensor.metricName = "temp_" + sensor.metricName
		}

		// Verify sensor file exists
		if _, err := os.ReadFile(file); err != nil {
			continue
		}
		sensor.file = file

		// Set default sensor tags
		sensor.tags = map[string]string{
			"type": "node",
		}
		// Apply tag override configuration if applicable
		for key, newtags := range m.config.TagOverride {
			if strings.Contains(sensor.file, key) {
				sensor.tags = newtags
				break
			}
		}

		// Read max temperature if enabled
		if m.config.ReportMaxTemp {
			maxTempFile := strings.TrimSuffix(file, "_input") + "_max"
			if buffer, err := os.ReadFile(maxTempFile); err == nil {
				if x, err := strconv.ParseInt(strings.TrimSpace(string(buffer)), 10, 64); err == nil {
					sensor.maxTempName = strings.Replace(sensor.metricName, "temp", "max_temp", 1)
					sensor.maxTemp = x / 1000
				}
			}
		}

		// Read critical temperature if enabled
		if m.config.ReportCriticalTemp {
			criticalTempFile := strings.TrimSuffix(file, "_input") + "_crit"
			if buffer, err := os.ReadFile(criticalTempFile); err == nil {
				if x, err := strconv.ParseInt(strings.TrimSpace(string(buffer)), 10, 64); err == nil {
					sensor.critTempName = strings.Replace(sensor.metricName, "temp", "crit_temp", 1)
					sensor.critTemp = x / 1000
				}
			}
		}

		m.sensors = append(m.sensors, sensor)
	}

	if len(m.sensors) == 0 {
		return fmt.Errorf("no temperature sensors found")
	}

	m.init = true
	return nil
}

func (m *TempCollector) Read(interval time.Duration, output chan lp.CCMessage) {
	// For each sensor, read temperature and send metric if allowed.
	for _, sensor := range m.sensors {
		// Read sensor file
		buffer, err := os.ReadFile(sensor.file)
		if err != nil {
			cclog.ComponentError(m.name, fmt.Sprintf("Read(): Failed to read file '%s': %v", sensor.file, err))
			continue
		}
		x, err := strconv.ParseInt(strings.TrimSpace(string(buffer)), 10, 64)
		if err != nil {
			cclog.ComponentError(m.name, fmt.Sprintf("Read(): Failed to convert temperature '%s' to int64: %v", buffer, err))
			continue
		}
		x /= 1000
		if m.shouldOutput(sensor.metricName) {
			y, err := lp.NewMessage(sensor.metricName, sensor.tags, m.meta, map[string]interface{}{"value": x}, time.Now())
			if err == nil {
				output <- y
			}
		}

		// Send max temperature if enabled and available
		if m.config.ReportMaxTemp && sensor.maxTemp != 0 && m.shouldOutput(sensor.maxTempName) {
			y, err := lp.NewMessage(sensor.maxTempName, sensor.tags, m.meta, map[string]interface{}{"value": sensor.maxTemp}, time.Now())
			if err == nil {
				output <- y
			}
		}

		// Send critical temperature if enabled and available
		if m.config.ReportCriticalTemp && sensor.critTemp != 0 && m.shouldOutput(sensor.critTempName) {
			y, err := lp.NewMessage(sensor.critTempName, sensor.tags, m.meta, map[string]interface{}{"value": sensor.critTemp}, time.Now())
			if err == nil {
				output <- y
			}
		}
	}
}

func (m *TempCollector) Close() {
	m.init = false
}
