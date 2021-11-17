package collectors

import (
	"fmt"
	lp "github.com/influxdata/line-protocol"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const HWMON_PATH = `/sys/class/hwmon`

type TempCollector struct {
	MetricCollector
	tag_override map[string]map[string]string // if outer key in path, use additional tags
}

func (m *TempCollector) Init() error {
	m.name = "TempCollector"
	m.setup()
	m.init = true
	//	m.tag_override = map[string]map[string]string{"hwmon0": {"type": "socket", "type-id": "0"},
	//		"hwmon1": {"type": "socket", "type-id": "1"}}
	m.tag_override = map[string]map[string]string{}
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

func (m *TempCollector) Read(interval time.Duration, out *[]lp.MutableMetric) {

	sensors, err := get_hwmon_sensors()
	if err != nil {
		return
	}
	for _, files := range sensors {
		for name, file := range files {
			tags := map[string]string{"type": "node"}
			for key, newtags := range m.tag_override {
				if strings.Contains(file, key) {
					tags = newtags
					break
				}
			}
			buffer, err := ioutil.ReadFile(string(file))
			if err != nil {
				continue
			}
			x, err := strconv.ParseInt(strings.Replace(string(buffer), "\n", "", -1), 0, 64)
			if err == nil {
				y, err := lp.New(strings.ToLower(name), tags, map[string]interface{}{"value": float64(x) / 1000}, time.Now())
				if err == nil {
					*out = append(*out, y)
				}
			}
		}
	}
}

func (m *TempCollector) Close() {
	m.init = false
	return
}
