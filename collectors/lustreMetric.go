package collectors

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	lp "github.com/ClusterCockpit/cc-metric-collector/internal/ccMetric"
)

const LUSTRE_SYSFS = `/sys/fs/lustre`
const LCTL_CMD = `lctl`
const LCTL_OPTION = `get_param`

type LustreCollectorConfig struct {
	LCtlCommand    string   `json:"lctl_command"`
	ExcludeMetrics []string `json:"exclude_metrics"`
}

type LustreCollector struct {
	metricCollector
	tags    map[string]string
	matches map[string]map[string]int
	devices []string
	config  LustreCollectorConfig
	lctl    string
}

func (m *LustreCollector) getDevices() []string {
	devices := make([]string, 0)

	// //Version reading devices from sysfs
	// globPattern := filepath.Join(LUSTRE_SYSFS, "llite/*/stats")
	// files, err := filepath.Glob(globPattern)
	// if err != nil {
	// 	return devices
	// }
	// for _, f := range files {
	// 	pathlist := strings.Split(f, "/")
	// 	devices = append(devices, pathlist[4])
	// }

	command := exec.Command(m.lctl, LCTL_OPTION, "llite.*.stats")
	command.Wait()
	stdout, err := command.Output()
	if err != nil {
		return devices
	}
	for _, line := range strings.Split(string(stdout), "\n") {
		if strings.HasPrefix(line, "llite") {
			linefields := strings.Split(line, ".")
			if len(linefields) > 2 {
				devices = append(devices, linefields[1])
			}
		}
	}
	return devices
}

// //Version reading the stats data of a device from sysfs
// func (m *LustreCollector) getDeviceDataSysfs(device string) []string {
// 	llitedir := filepath.Join(LUSTRE_SYSFS, "llite")
// 	devdir := filepath.Join(llitedir, device)
// 	statsfile := filepath.Join(devdir, "stats")
// 	buffer, err := ioutil.ReadFile(statsfile)
// 	if err != nil {
// 		return make([]string, 0)
// 	}
// 	return strings.Split(string(buffer), "\n")
// }

func (m *LustreCollector) getDeviceDataCommand(device string) []string {
	statsfile := fmt.Sprintf("llite.%s.stats", device)
	command := exec.Command(m.lctl, LCTL_OPTION, statsfile)
	command.Wait()
	stdout, _ := command.Output()
	return strings.Split(string(stdout), "\n")
}

func (m *LustreCollector) Init(config json.RawMessage) error {
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
	m.meta = map[string]string{"source": m.name, "group": "Lustre"}
	m.matches = map[string]map[string]int{"read_bytes": {"read_bytes": 6, "read_requests": 1},
		"write_bytes":      {"write_bytes": 6, "write_requests": 1},
		"open":             {"open": 1},
		"close":            {"close": 1},
		"setattr":          {"setattr": 1},
		"getattr":          {"getattr": 1},
		"statfs":           {"statfs": 1},
		"inode_permission": {"inode_permission": 1}}
	p, err := exec.LookPath(m.config.LCtlCommand)
	if err != nil {
		p, err = exec.LookPath(LCTL_CMD)
		if err != nil {
			return err
		}
	}
	m.lctl = p

	m.devices = m.getDevices()
	if len(m.devices) == 0 {
		return errors.New("no metrics to collect")
	}
	m.init = true
	return nil
}

func (m *LustreCollector) Read(interval time.Duration, output chan lp.CCMetric) {
	if !m.init {
		return
	}
	for _, device := range m.devices {
		stats := m.getDeviceDataCommand(device)

		for _, line := range stats {
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
								y, err := lp.New(name, m.tags, m.meta, map[string]interface{}{"value": x}, time.Now())
								if err == nil {
									if strings.Contains(name, "byte") {
										y.AddMeta("unit", "Byte")
									}
									output <- y
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
