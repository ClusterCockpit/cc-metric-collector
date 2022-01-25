package collectors

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"strconv"
	"strings"
	"time"

	lp "github.com/influxdata/line-protocol"
)

const LUSTREFILE = `/proc/fs/lustre/llite/lnec-XXXXXX/stats`

type LustreCollectorConfig struct {
	Procfiles      []string `json:"procfiles"`
	ExcludeMetrics []string `json:"exclude_metrics"`
}

type LustreCollector struct {
	MetricCollector
	tags    map[string]string
	matches map[string]map[string]int
	devices []string
	config  LustreCollectorConfig
}

func (m *LustreCollector) Init(config []byte) error {
	var err error
	m.name = "LustreCollector"
	if len(config) > 0 {
		err = json.Unmarshal(config, &m.config)
		if err != nil {
			return err
		}
	}
	m.setup()
	m.tags = map[string]string{"type": "node"}
	m.matches = map[string]map[string]int{"read_bytes": {"read_bytes": 6, "read_requests": 1},
		"write_bytes":      {"write_bytes": 6, "write_requests": 1},
		"open":             {"open": 1},
		"close":            {"close": 1},
		"setattr":          {"setattr": 1},
		"getattr":          {"getattr": 1},
		"statfs":           {"statfs": 1},
		"inode_permission": {"inode_permission": 1}}
	m.devices = make([]string, 0)
	for _, p := range m.config.Procfiles {
		_, err := ioutil.ReadFile(p)
		if err == nil {
			m.devices = append(m.devices, p)
		} else {
			log.Print(err.Error())
			continue
		}
	}

	if len(m.devices) == 0 {
		return errors.New("No metrics to collect")
	}
	m.init = true
	return nil
}

func (m *LustreCollector) Read(interval time.Duration, out *[]lp.MutableMetric) {
	if !m.init {
		return
	}
	for _, p := range m.devices {
		buffer, err := ioutil.ReadFile(p)

		if err != nil {
			log.Print(err)
			return
		}

		for _, line := range strings.Split(string(buffer), "\n") {
			lf := strings.Fields(line)
			if len(lf) > 1 {
				for match, fields := range m.matches {
					if lf[0] == match {
						for name, idx := range fields {
							_, skip := stringArrayContains(m.config.ExcludeMetrics, name)
							if skip {
								continue
							}
							x, err := strconv.ParseInt(lf[idx], 0, 64)
							if err == nil {
								y, err := lp.New(name, m.tags, map[string]interface{}{"value": x}, time.Now())
								if err == nil {
									*out = append(*out, y)
								}
							}
						}
					}
				}
			}
		}
	}
}

func (m *LustreCollector) Close() {
	m.init = false
}
