package collectors

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"os/user"
	"strconv"
	"strings"
	"time"

	cclog "github.com/ClusterCockpit/cc-metric-collector/internal/ccLogger"
	lp "github.com/ClusterCockpit/cc-metric-collector/internal/ccMetric"
)

const LUSTRE_SYSFS = `/sys/fs/lustre`
const LCTL_CMD = `lctl`
const LCTL_OPTION = `get_param`

type LustreCollectorConfig struct {
	LCtlCommand    string   `json:"lctl_command"`
	ExcludeMetrics []string `json:"exclude_metrics"`
	SendAllMetrics bool     `json:"send_all_metrics"`
	Sudo           bool     `json:"use_sudo"`
}

type LustreCollector struct {
	metricCollector
	tags    map[string]string
	matches map[string]map[string]int
	stats   map[string]map[string]int64
	config  LustreCollectorConfig
	lctl    string
	sudoCmd string
}

func (m *LustreCollector) getDeviceDataCommand(device string) []string {
	var command *exec.Cmd
	statsfile := fmt.Sprintf("llite.%s.stats", device)
	if m.config.Sudo {
		command = exec.Command(m.sudoCmd, m.lctl, LCTL_OPTION, statsfile)
	} else {
		command = exec.Command(m.lctl, LCTL_OPTION, statsfile)
	}
	command.Wait()
	stdout, _ := command.Output()
	return strings.Split(string(stdout), "\n")
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

	data := m.getDeviceDataCommand("*")

	for _, line := range data {
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
	defmatches := map[string]map[string]int{
		"read_bytes":       {"lustre_read_bytes": 6, "lustre_read_requests": 1},
		"write_bytes":      {"lustre_write_bytes": 6, "lustre_write_requests": 1},
		"open":             {"lustre_open": 1},
		"close":            {"lustre_close": 1},
		"setattr":          {"lustre_setattr": 1},
		"getattr":          {"lustre_getattr": 1},
		"statfs":           {"lustre_statfs": 1},
		"inode_permission": {"lustre_inode_permission": 1}}

	// Lustre file system statistics can only be queried by user root
	if !m.config.Sudo {
		user, err := user.Current()
		if err != nil {
			cclog.ComponentError(m.name, "Failed to get current user:", err.Error())
			return err
		}
		if user.Uid != "0" {
			cclog.ComponentError(m.name, "Lustre file system statistics can only be queried by user root")
			return err
		}
	}

	m.matches = make(map[string]map[string]int)
	for lineprefix, names := range defmatches {
		for metricname, offset := range names {
			_, skip := stringArrayContains(m.config.ExcludeMetrics, metricname)
			if skip {
				continue
			}
			if _, prefixExist := m.matches[lineprefix]; !prefixExist {
				m.matches[lineprefix] = make(map[string]int)
			}
			if _, metricExist := m.matches[lineprefix][metricname]; !metricExist {
				m.matches[lineprefix][metricname] = offset
			}
		}
	}
	p, err := exec.LookPath(m.config.LCtlCommand)
	if err != nil {
		p, err = exec.LookPath(LCTL_CMD)
		if err != nil {
			return err
		}
	}
	m.lctl = p
	if m.config.Sudo {
		p, err := exec.LookPath("sudo")
		if err != nil {
			m.sudoCmd = p
		}
	}

	devices := m.getDevices()
	if len(devices) == 0 {
		return errors.New("no metrics to collect")
	}
	m.stats = make(map[string]map[string]int64)
	for _, d := range devices {
		m.stats[d] = make(map[string]int64)
		for _, names := range m.matches {
			for metricname := range names {
				m.stats[d][metricname] = 0
			}
		}
	}
	m.init = true
	return nil
}

func (m *LustreCollector) Read(interval time.Duration, output chan lp.CCMetric) {
	if !m.init {
		return
	}
	for device, devData := range m.stats {
		stats := m.getDeviceDataCommand(device)
		processed := []string{}

		for _, line := range stats {
			lf := strings.Fields(line)
			if len(lf) > 1 {
				if fields, ok := m.matches[lf[0]]; ok {
					for name, idx := range fields {
						x, err := strconv.ParseInt(lf[idx], 0, 64)
						if err != nil {
							continue
						}
						value := x - devData[name]
						devData[name] = x
						if value < 0 {
							value = 0
						}
						y, err := lp.New(name, m.tags, m.meta, map[string]interface{}{"value": value}, time.Now())
						if err == nil {
							y.AddTag("device", device)
							if strings.Contains(name, "byte") {
								y.AddMeta("unit", "Byte")
							}
							output <- y
							if m.config.SendAllMetrics {
								processed = append(processed, name)
							}
						}
					}
				}
			}
		}
		if m.config.SendAllMetrics {
			for name := range devData {
				if _, done := stringArrayContains(processed, name); !done {
					y, err := lp.New(name, m.tags, m.meta, map[string]interface{}{"value": 0}, time.Now())
					if err == nil {
						y.AddTag("device", device)
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

func (m *LustreCollector) Close() {
	m.init = false
}
