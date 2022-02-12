package collectors

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	cclog "github.com/ClusterCockpit/cc-metric-collector/internal/ccLogger"
	lp "github.com/ClusterCockpit/cc-metric-collector/internal/ccMetric"
)

const HWMON_PATH = `/sys/class/hwmon`

type TempCollectorConfig struct {
	ExcludeMetrics []string                     `json:"exclude_metrics"`
	TagOverride    map[string]map[string]string `json:"tag_override"`
}

type TempCollector struct {
	metricCollector
	config  TempCollectorConfig
	sensors map[string]string
}

func (m *TempCollector) Init(config json.RawMessage) error {
	m.name = "TempCollector"
	m.setup()
	m.meta = map[string]string{"source": m.name, "group": "IPMI", "unit": "degC"}
	if len(config) > 0 {
		err := json.Unmarshal(config, &m.config)
		if err != nil {
			return err
		}
	}

	// Find all temperature sensor files
	m.sensors = make(map[string]string)
	globPattern := filepath.Join(HWMON_PATH, "*", "temp*_input")
	inputFiles, err := filepath.Glob(globPattern)
	if err != nil {
		return fmt.Errorf("Unable to glob files with pattern '%s': %v", globPattern, err)
	}
	if inputFiles == nil {
		return fmt.Errorf("Unable to find any files with pattern '%s'", globPattern)
	}

	// Get sensor name for each temperature sensor file
	for _, file := range inputFiles {
		nameFile := filepath.Join(filepath.Dir(file), "name")
		name := ""
		n, err := ioutil.ReadFile(nameFile)
		if err == nil {
			name = strings.TrimSpace(string(n))
		}
		labelFile := strings.TrimSuffix(file, "_input") + "_label"
		label := ""
		l, err := ioutil.ReadFile(labelFile)
		if err == nil {
			label = strings.TrimSpace(string(l))
		}
		metricName := ""
		switch {
		case len(name) == 0 && len(label) == 0:
			continue
		case len(name) != 0 && len(label) != 0:
			metricName = name + "_" + label
		case len(name) != 0:
			metricName = name
		case len(label) != 0:
			metricName = label
		}
		metricName = strings.Replace(metricName, " ", "_", -1)
		if !strings.Contains(metricName, "temp") {
			metricName = "temp_" + metricName
		}
		metricName = strings.ToLower(metricName)
		m.sensors[metricName] = file
	}

	// Empty sensors map
	if len(m.sensors) == 0 {
		return fmt.Errorf("No temperature sensors found")
	}

	// Finished initialization
	m.init = true
	return nil
}

func (m *TempCollector) Read(interval time.Duration, output chan lp.CCMetric) {

	for metricName, file := range m.sensors {
		tags := map[string]string{"type": "node"}
		for key, newtags := range m.config.TagOverride {
			if strings.Contains(file, key) {
				tags = newtags
				break
			}
		}
		buffer, err := ioutil.ReadFile(file)
		if err != nil {
			continue
		}
		x, err := strconv.ParseInt(strings.TrimSpace(string(buffer)), 10, 64)
		if err == nil {
			y, err := lp.New(metricName, tags, m.meta, map[string]interface{}{"value": int(float64(x) / 1000)}, time.Now())
			if err == nil {
				cclog.ComponentDebug(m.name, y)
				output <- y
			}
		}
	}

}

func (m *TempCollector) Close() {
	m.init = false
}
