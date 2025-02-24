package collectors

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	cclog "github.com/ClusterCockpit/cc-metric-collector/pkg/ccLogger"
	lp "github.com/ClusterCockpit/cc-lib/ccMessage"
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
	metricName   string // Default: name_label
	file         string
	maxTempName  string
	maxTemp      int64
	critTempName string
	critTemp     int64
	tags         map[string]string
}

type TempCollector struct {
	metricCollector
	config struct {
		ExcludeMetrics     []string                     `json:"exclude_metrics"`
		TagOverride        map[string]map[string]string `json:"tag_override"`
		ReportMaxTemp      bool                         `json:"report_max_temperature"`
		ReportCriticalTemp bool                         `json:"report_critical_temperature"`
	}
	sensors []*TempCollectorSensor
}

func (m *TempCollector) Init(config json.RawMessage) error {
	// Check if already initialized
	if m.init {
		return nil
	}

	m.name = "TempCollector"
	m.parallel = true
	m.setup()
	if len(config) > 0 {
		err := json.Unmarshal(config, &m.config)
		if err != nil {
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

		// sensor name
		nameFile := filepath.Join(filepath.Dir(file), "name")
		name, err := os.ReadFile(nameFile)
		if err == nil {
			sensor.name = strings.TrimSpace(string(name))
		}

		// sensor label
		labelFile := strings.TrimSuffix(file, "_input") + "_label"
		label, err := os.ReadFile(labelFile)
		if err == nil {
			sensor.label = strings.TrimSpace(string(label))
		}

		// sensor metric name
		switch {
		case len(sensor.name) == 0 && len(sensor.label) == 0:
			continue
		case sensor.name == "coretemp" && strings.HasPrefix(sensor.label, "Core ") ||
			sensor.name == "coretemp" && strings.HasPrefix(sensor.label, "Package id "):
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
		// Add temperature prefix, if required
		if !strings.Contains(sensor.metricName, "temp") {
			sensor.metricName = "temp_" + sensor.metricName
		}

		// Sensor file
		_, err = os.ReadFile(file)
		if err != nil {
			continue
		}
		sensor.file = file

		// Sensor tags
		sensor.tags = map[string]string{
			"type": "node",
		}

		// Apply tag override configuration
		for key, newtags := range m.config.TagOverride {
			if strings.Contains(sensor.file, key) {
				sensor.tags = newtags
				break
			}
		}

		// max temperature
		if m.config.ReportMaxTemp {
			maxTempFile := strings.TrimSuffix(file, "_input") + "_max"
			if buffer, err := os.ReadFile(maxTempFile); err == nil {
				if x, err := strconv.ParseInt(strings.TrimSpace(string(buffer)), 10, 64); err == nil {
					sensor.maxTempName = strings.Replace(sensor.metricName, "temp", "max_temp", 1)
					sensor.maxTemp = x / 1000
				}
			}
		}

		// critical temperature
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

	// Empty sensors map
	if len(m.sensors) == 0 {
		return fmt.Errorf("no temperature sensors found")
	}

	// Finished initialization
	m.init = true
	return nil
}

func (m *TempCollector) Read(interval time.Duration, output chan lp.CCMessage) {

	for _, sensor := range m.sensors {
		// Read sensor file
		buffer, err := os.ReadFile(sensor.file)
		if err != nil {
			cclog.ComponentError(
				m.name,
				fmt.Sprintf("Read(): Failed to read file '%s': %v", sensor.file, err))
			continue
		}
		x, err := strconv.ParseInt(strings.TrimSpace(string(buffer)), 10, 64)
		if err != nil {
			cclog.ComponentError(
				m.name,
				fmt.Sprintf("Read(): Failed to convert temperature '%s' to int64: %v", buffer, err))
			continue
		}
		x /= 1000
		y, err := lp.NewMessage(
			sensor.metricName,
			sensor.tags,
			m.meta,
			map[string]interface{}{"value": x},
			time.Now(),
		)
		if err == nil {
			output <- y
		}

		// max temperature
		if m.config.ReportMaxTemp && sensor.maxTemp != 0 {
			y, err := lp.NewMessage(
				sensor.maxTempName,
				sensor.tags,
				m.meta,
				map[string]interface{}{"value": sensor.maxTemp},
				time.Now(),
			)
			if err == nil {
				output <- y
			}
		}

		// critical temperature
		if m.config.ReportCriticalTemp && sensor.critTemp != 0 {
			y, err := lp.NewMessage(
				sensor.critTempName,
				sensor.tags,
				m.meta,
				map[string]interface{}{"value": sensor.critTemp},
				time.Now(),
			)
			if err == nil {
				output <- y
			}
		}
	}

}

func (m *TempCollector) Close() {
	m.init = false
}
