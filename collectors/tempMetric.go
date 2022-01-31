package collectors

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
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
	config TempCollectorConfig
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
	m.init = true
	return nil
}

func get_hwmon_sensors() (map[string]map[string]string, error) {
	var folders []string
	var sensors map[string]map[string]string
	sensors = make(map[string]map[string]string)
	err := filepath.Walk(HWMON_PATH, func(p string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		folders = append(folders, p)
		return nil
	})
	if err != nil {
		return sensors, err
	}

	for _, f := range folders {
		sensors[f] = make(map[string]string)
		myp := fmt.Sprintf("%s/", f)
		err := filepath.Walk(myp, func(path string, info os.FileInfo, err error) error {
			dir, fname := filepath.Split(path)
			if strings.Contains(fname, "temp") && strings.Contains(fname, "_input") {
				namefile := fmt.Sprintf("%s/%s", dir, strings.Replace(fname, "_input", "_label", -1))
				name, ierr := ioutil.ReadFile(namefile)
				if ierr == nil {
					sensors[f][strings.Replace(string(name), "\n", "", -1)] = path
				}
			}
			return nil
		})
		if err != nil {
			continue
		}
	}
	return sensors, nil
}

func (m *TempCollector) Read(interval time.Duration, output chan *lp.CCMetric) {

	sensors, err := get_hwmon_sensors()
	if err != nil {
		return
	}
	for _, files := range sensors {
		for name, file := range files {
			tags := map[string]string{"type": "node"}
			for key, newtags := range m.config.TagOverride {
				if strings.Contains(file, key) {
					tags = newtags
					break
				}
			}
			mname := strings.Replace(name, " ", "_", -1)
			if !strings.Contains(mname, "temp") {
				mname = fmt.Sprintf("temp_%s", mname)
			}
			buffer, err := ioutil.ReadFile(string(file))
			if err != nil {
				continue
			}
			x, err := strconv.ParseInt(strings.Replace(string(buffer), "\n", "", -1), 0, 64)
			if err == nil {
				y, err := lp.New(strings.ToLower(mname), tags, m.meta, map[string]interface{}{"value": int(float64(x) / 1000)}, time.Now())
				if err == nil {
					cclog.ComponentDebug(m.name, y)
					output <- &y
				}
			}
		}
	}
}

func (m *TempCollector) Close() {
	m.init = false
}
