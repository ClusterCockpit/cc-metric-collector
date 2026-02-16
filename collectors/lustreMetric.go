// Copyright (C) NHR@FAU, University Erlangen-Nuremberg.
// All rights reserved. This file is part of cc-lib.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.
// additional authors:
// Holger Obermaier (NHR@KIT)

package collectors

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"os/user"
	"slices"
	"strconv"
	"strings"
	"time"

	cclog "github.com/ClusterCockpit/cc-lib/v2/ccLogger"
	lp "github.com/ClusterCockpit/cc-lib/v2/ccMessage"
)

const (
	LUSTRE_SYSFS = `/sys/fs/lustre`
	LCTL_CMD     = `lctl`
	LCTL_OPTION  = `get_param`
)

type LustreCollectorConfig struct {
	LCtlCommand        string   `json:"lctl_command,omitempty"`
	ExcludeMetrics     []string `json:"exclude_metrics,omitempty"`
	Sudo               bool     `json:"use_sudo,omitempty"`
	SendAbsoluteValues bool     `json:"send_abs_values,omitempty"`
	SendDerivedValues  bool     `json:"send_derived_values,omitempty"`
	SendDiffValues     bool     `json:"send_diff_values,omitempty"`
}

type LustreMetricDefinition struct {
	name       string
	lineprefix string
	lineoffset int
	unit       string
	calc       string
}

type LustreCollector struct {
	metricCollector

	tags          map[string]string
	config        LustreCollectorConfig
	lctl          string
	sudoCmd       string
	lastTimestamp time.Time                   // Store time stamp of last tick to derive bandwidths
	definitions   []LustreMetricDefinition    // Combined list without excluded metrics
	stats         map[string]map[string]int64 // Data for last value per device and metric
}

func (m *LustreCollector) getDeviceDataCommand(device string) []string {
	var command *exec.Cmd
	statsfile := fmt.Sprintf("llite.%s.stats", device)
	if m.config.Sudo {
		command = exec.Command(m.sudoCmd, m.lctl, LCTL_OPTION, statsfile)
	} else {
		command = exec.Command(m.lctl, LCTL_OPTION, statsfile)
	}
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

func getMetricData(lines []string, prefix string, offset int) (int64, error) {
	for _, line := range lines {
		if strings.HasPrefix(line, prefix) {
			lf := strings.Fields(line)
			return strconv.ParseInt(lf[offset], 0, 64)
		}
	}
	return 0, errors.New("no such line in data")
}

// //Version reading the stats data of a device from sysfs
// func (m *LustreCollector) getDeviceDataSysfs(device string) []string {
// 	llitedir := filepath.Join(LUSTRE_SYSFS, "llite")
// 	devdir := filepath.Join(llitedir, device)
// 	statsfile := filepath.Join(devdir, "stats")
// 	buffer, err := os.ReadFile(statsfile)
// 	if err != nil {
// 		return make([]string, 0)
// 	}
// 	return strings.Split(string(buffer), "\n")
// }

var LustreAbsMetrics = []LustreMetricDefinition{
	{
		name:       "lustre_read_requests",
		lineprefix: "read_bytes",
		lineoffset: 1,
		unit:       "requests",
		calc:       "none",
	},
	{
		name:       "lustre_write_requests",
		lineprefix: "write_bytes",
		lineoffset: 1,
		unit:       "requests",
		calc:       "none",
	},
	{
		name:       "lustre_read_bytes",
		lineprefix: "read_bytes",
		lineoffset: 6,
		unit:       "bytes",
		calc:       "none",
	},
	{
		name:       "lustre_write_bytes",
		lineprefix: "write_bytes",
		lineoffset: 6,
		unit:       "bytes",
		calc:       "none",
	},
	{
		name:       "lustre_open",
		lineprefix: "open",
		lineoffset: 1,
		unit:       "",
		calc:       "none",
	},
	{
		name:       "lustre_close",
		lineprefix: "close",
		lineoffset: 1,
		unit:       "",
		calc:       "none",
	},
	{
		name:       "lustre_setattr",
		lineprefix: "setattr",
		lineoffset: 1,
		unit:       "",
		calc:       "none",
	},
	{
		name:       "lustre_getattr",
		lineprefix: "getattr",
		lineoffset: 1,
		unit:       "",
		calc:       "none",
	},
	{
		name:       "lustre_statfs",
		lineprefix: "statfs",
		lineoffset: 1,
		unit:       "",
		calc:       "none",
	},
	{
		name:       "lustre_inode_permission",
		lineprefix: "inode_permission",
		lineoffset: 1,
		unit:       "",
		calc:       "none",
	},
}

var LustreDiffMetrics = []LustreMetricDefinition{
	{
		name:       "lustre_read_requests_diff",
		lineprefix: "read_bytes",
		lineoffset: 1,
		unit:       "requests",
		calc:       "difference",
	},
	{
		name:       "lustre_write_requests_diff",
		lineprefix: "write_bytes",
		lineoffset: 1,
		unit:       "requests",
		calc:       "difference",
	},
	{
		name:       "lustre_read_bytes_diff",
		lineprefix: "read_bytes",
		lineoffset: 6,
		unit:       "bytes",
		calc:       "difference",
	},
	{
		name:       "lustre_write_bytes_diff",
		lineprefix: "write_bytes",
		lineoffset: 6,
		unit:       "bytes",
		calc:       "difference",
	},
	{
		name:       "lustre_open_diff",
		lineprefix: "open",
		lineoffset: 1,
		unit:       "",
		calc:       "difference",
	},
	{
		name:       "lustre_close_diff",
		lineprefix: "close",
		lineoffset: 1,
		unit:       "",
		calc:       "difference",
	},
	{
		name:       "lustre_setattr_diff",
		lineprefix: "setattr",
		lineoffset: 1,
		unit:       "",
		calc:       "difference",
	},
	{
		name:       "lustre_getattr_diff",
		lineprefix: "getattr",
		lineoffset: 1,
		unit:       "",
		calc:       "difference",
	},
	{
		name:       "lustre_statfs_diff",
		lineprefix: "statfs",
		lineoffset: 1,
		unit:       "",
		calc:       "difference",
	},
	{
		name:       "lustre_inode_permission_diff",
		lineprefix: "inode_permission",
		lineoffset: 1,
		unit:       "",
		calc:       "difference",
	},
}

var LustreDeriveMetrics = []LustreMetricDefinition{
	{
		name:       "lustre_read_requests_rate",
		lineprefix: "read_bytes",
		lineoffset: 1,
		unit:       "requests/sec",
		calc:       "derivative",
	},
	{
		name:       "lustre_write_requests_rate",
		lineprefix: "write_bytes",
		lineoffset: 1,
		unit:       "requests/sec",
		calc:       "derivative",
	},
	{
		name:       "lustre_read_bw",
		lineprefix: "read_bytes",
		lineoffset: 6,
		unit:       "bytes/sec",
		calc:       "derivative",
	},
	{
		name:       "lustre_write_bw",
		lineprefix: "write_bytes",
		lineoffset: 6,
		unit:       "bytes/sec",
		calc:       "derivative",
	},
}

func (m *LustreCollector) Init(config json.RawMessage) error {
	var err error
	m.name = "LustreCollector"
	m.parallel = true
	if len(config) > 0 {
		err = json.Unmarshal(config, &m.config)
		if err != nil {
			return err
		}
	}
	if err := m.setup(); err != nil {
		return fmt.Errorf("%s Init(): setup() call failed: %w", m.name, err)
	}
	m.tags = map[string]string{"type": "node"}
	m.meta = map[string]string{"source": m.name, "group": "Lustre"}

	// Lustre file system statistics can only be queried by user root
	// or with password-less sudo
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
	} else {
		p, err := exec.LookPath("sudo")
		if err != nil {
			cclog.ComponentError(m.name, "Cannot find 'sudo'")
			return err
		}
		m.sudoCmd = p
	}

	p, err := exec.LookPath(m.config.LCtlCommand)
	if err != nil {
		p, err = exec.LookPath(LCTL_CMD)
		if err != nil {
			return err
		}
	}
	m.lctl = p

	m.definitions = []LustreMetricDefinition{}
	if m.config.SendAbsoluteValues {
		for _, def := range LustreAbsMetrics {
			if !slices.Contains(m.config.ExcludeMetrics, def.name) {
				m.definitions = append(m.definitions, def)
			}
		}
	}
	if m.config.SendDiffValues {
		for _, def := range LustreDiffMetrics {
			if !slices.Contains(m.config.ExcludeMetrics, def.name) {
				m.definitions = append(m.definitions, def)
			}
		}
	}
	if m.config.SendDerivedValues {
		for _, def := range LustreDeriveMetrics {
			if !slices.Contains(m.config.ExcludeMetrics, def.name) {
				m.definitions = append(m.definitions, def)
			}
		}
	}
	if len(m.definitions) == 0 {
		return errors.New("no metrics to collect")
	}

	devices := m.getDevices()
	if len(devices) == 0 {
		return errors.New("no Lustre devices found")
	}
	m.stats = make(map[string]map[string]int64)
	for _, d := range devices {
		m.stats[d] = make(map[string]int64)
		data := m.getDeviceDataCommand(d)
		for _, def := range m.definitions {
			x, err := getMetricData(data, def.lineprefix, def.lineoffset)
			if err == nil {
				m.stats[d][def.name] = x
			} else {
				m.stats[d][def.name] = 0
			}
		}
	}
	m.lastTimestamp = time.Now()
	m.init = true
	return nil
}

func (m *LustreCollector) Read(interval time.Duration, output chan lp.CCMessage) {
	if !m.init {
		return
	}
	now := time.Now()
	tdiff := now.Sub(m.lastTimestamp)
	for device, devData := range m.stats {
		data := m.getDeviceDataCommand(device)
		for _, def := range m.definitions {
			var use_x int64
			var err error
			var y lp.CCMessage
			x, err := getMetricData(data, def.lineprefix, def.lineoffset)
			if err == nil {
				use_x = x
			} else {
				use_x = devData[def.name]
			}
			var value any
			switch def.calc {
			case "none":
				value = use_x
				y, err = lp.NewMessage(def.name, m.tags, m.meta, map[string]any{"value": value}, time.Now())
			case "difference":
				value = use_x - devData[def.name]
				if value.(int64) < 0 {
					value = 0
				}
				y, err = lp.NewMessage(def.name, m.tags, m.meta, map[string]any{"value": value}, time.Now())
			case "derivative":
				value = float64(use_x-devData[def.name]) / tdiff.Seconds()
				if value.(float64) < 0 {
					value = 0
				}
				y, err = lp.NewMessage(def.name, m.tags, m.meta, map[string]any{"value": value}, time.Now())
			}
			if err == nil {
				y.AddTag("device", device)
				if len(def.unit) > 0 {
					y.AddMeta("unit", def.unit)
				}
				output <- y
			}
			devData[def.name] = use_x
		}
	}
	m.lastTimestamp = now
}

func (m *LustreCollector) Close() {
	m.init = false
}
