// Copyright (C) NHR@FAU, University Erlangen-Nuremberg.
// All rights reserved. This file is part of cc-lib.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.
// additional authors:
// Holger Obermaier (NHR@KIT)

package collectors

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os/exec"
	"os/user"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	cclog "github.com/ClusterCockpit/cc-lib/v2/ccLogger"
	lp "github.com/ClusterCockpit/cc-lib/v2/ccMessage"
)

const DEFAULT_GPFS_CMD = "mmpmon"

type GpfsCollectorState map[string]int64

type GpfsCollectorConfig struct {
	Mmpmon             string   `json:"mmpmon_path,omitempty"`
	ExcludeFilesystem  []string `json:"exclude_filesystem,omitempty"`
	ExcludeMetrics     []string `json:"exclude_metrics,omitempty"`
	Sudo               bool     `json:"use_sudo,omitempty"`
	SendAbsoluteValues bool     `json:"send_abs_values,omitempty"`
	SendDiffValues     bool     `json:"send_diff_values,omitempty"`
	SendDerivedValues  bool     `json:"send_derived_values,omitempty"`
	SendTotalValues    bool     `json:"send_total_values,omitempty"`
	SendBandwidths     bool     `json:"send_bandwidths,omitempty"`
}

type GpfsMetricDefinition struct {
	name   string
	desc   string
	prefix string
	unit   string
	calc   string
}

type GpfsCollector struct {
	metricCollector
	tags          map[string]string
	config        GpfsCollectorConfig
	sudoCmd       string
	skipFS        map[string]struct{}
	lastTimestamp map[string]time.Time          // Store timestamp of lastState per filesystem to derive bandwidths
	definitions   []GpfsMetricDefinition        // all metrics to report
	lastState     map[string]GpfsCollectorState // one GpfsCollectorState per filesystem
}

var GpfsAbsMetrics = []GpfsMetricDefinition{
	{
		name:   "gpfs_num_opens",
		desc:   "number of opens",
		prefix: "_oc_",
		unit:   "requests",
		calc:   "none",
	},
	{
		name:   "gpfs_num_closes",
		desc:   "number of closes",
		prefix: "_cc_",
		unit:   "requests",
		calc:   "none",
	},
	{
		name:   "gpfs_num_reads",
		desc:   "number of reads",
		prefix: "_rdc_",
		unit:   "requests",
		calc:   "none",
	},
	{
		name:   "gpfs_num_writes",
		desc:   "number of writes",
		prefix: "_wc_",
		unit:   "requests",
		calc:   "none",
	},
	{
		name:   "gpfs_num_readdirs",
		desc:   "number of readdirs",
		prefix: "_dir_",
		unit:   "requests",
		calc:   "none",
	},
	{
		name:   "gpfs_num_inode_updates",
		desc:   "number of Inode Updates",
		prefix: "_iu_",
		unit:   "requests",
		calc:   "none",
	},
	{
		name:   "gpfs_bytes_read",
		desc:   "bytes read",
		prefix: "_br_",
		unit:   "bytes",
		calc:   "none",
	},
	{
		name:   "gpfs_bytes_written",
		desc:   "bytes written",
		prefix: "_bw_",
		unit:   "bytes",
		calc:   "none",
	},
}

var GpfsDiffMetrics = []GpfsMetricDefinition{
	{
		name:   "gpfs_num_opens_diff",
		desc:   "number of opens (diff)",
		prefix: "_oc_",
		unit:   "requests",
		calc:   "difference",
	},
	{
		name:   "gpfs_num_closes_diff",
		desc:   "number of closes (diff)",
		prefix: "_cc_",
		unit:   "requests",
		calc:   "difference",
	},
	{
		name:   "gpfs_num_reads_diff",
		desc:   "number of reads (diff)",
		prefix: "_rdc_",
		unit:   "requests",
		calc:   "difference",
	},
	{
		name:   "gpfs_num_writes_diff",
		desc:   "number of writes (diff)",
		prefix: "_wc_",
		unit:   "requests",
		calc:   "difference",
	},
	{
		name:   "gpfs_num_readdirs_diff",
		desc:   "number of readdirs (diff)",
		prefix: "_dir_",
		unit:   "requests",
		calc:   "difference",
	},
	{
		name:   "gpfs_num_inode_updates_diff",
		desc:   "number of Inode Updates (diff)",
		prefix: "_iu_",
		unit:   "requests",
		calc:   "difference",
	},
	{
		name:   "gpfs_bytes_read_diff",
		desc:   "bytes read (diff)",
		prefix: "_br_",
		unit:   "bytes",
		calc:   "difference",
	},
	{
		name:   "gpfs_bytes_written_diff",
		desc:   "bytes written (diff)",
		prefix: "_bw_",
		unit:   "bytes",
		calc:   "difference",
	},
}

var GpfsDeriveMetrics = []GpfsMetricDefinition{
	{
		name:   "gpfs_opens_rate",
		desc:   "number of opens (rate)",
		prefix: "_oc_",
		unit:   "requests/sec",
		calc:   "derivative",
	},
	{
		name:   "gpfs_closes_rate",
		desc:   "number of closes (rate)",
		prefix: "_oc_",
		unit:   "requests/sec",
		calc:   "derivative",
	},
	{
		name:   "gpfs_reads_rate",
		desc:   "number of reads (rate)",
		prefix: "_rdc_",
		unit:   "requests/sec",
		calc:   "derivative",
	},
	{
		name:   "gpfs_writes_rate",
		desc:   "number of writes (rate)",
		prefix: "_wc_",
		unit:   "requests/sec",
		calc:   "derivative",
	},
	{
		name:   "gpfs_readdirs_rate",
		desc:   "number of readdirs (rate)",
		prefix: "_dir_",
		unit:   "requests/sec",
		calc:   "derivative",
	},
	{
		name:   "gpfs_inode_updates_rate",
		desc:   "number of Inode Updates (rate)",
		prefix: "_iu_",
		unit:   "requests/sec",
		calc:   "derivative",
	},
	{
		name:   "gpfs_bw_read",
		desc:   "bytes read (rate)",
		prefix: "_br_",
		unit:   "bytes/sec",
		calc:   "derivative",
	},
	{
		name:   "gpfs_bw_write",
		desc:   "bytes written (rate)",
		prefix: "_bw_",
		unit:   "bytes/sec",
		calc:   "derivative",
	},
}

var GpfsTotalMetrics = []GpfsMetricDefinition{
	{
		name:   "gpfs_bytes_total",
		desc:   "bytes total",
		prefix: "bytesTotal",
		unit:   "bytes",
		calc:   "none",
	},
	{
		name:   "gpfs_bytes_total_diff",
		desc:   "bytes total (diff)",
		prefix: "bytesTotal",
		unit:   "bytes",
		calc:   "difference",
	},
	{
		name:   "gpfs_bw_total",
		desc:   "bytes total (rate)",
		prefix: "bytesTotal",
		unit:   "bytes/sec",
		calc:   "derivative",
	},
	{
		name:   "gpfs_iops",
		desc:   "iops",
		prefix: "iops",
		unit:   "requests",
		calc:   "none",
	},
	{
		name:   "gpfs_iops_diff",
		desc:   "iops  (diff)",
		prefix: "iops",
		unit:   "requests",
		calc:   "difference",
	},
	{
		name:   "gpfs_iops_rate",
		desc:   "iops (rate)",
		prefix: "iops",
		unit:   "requests/sec",
		calc:   "derivative",
	},
	{
		name:   "gpfs_metaops",
		desc:   "metaops",
		prefix: "metaops",
		unit:   "requests",
		calc:   "none",
	},
	{
		name:   "gpfs_metaops_diff",
		desc:   "metaops (diff)",
		prefix: "metaops",
		unit:   "requests",
		calc:   "difference",
	},
	{
		name:   "gpfs_metaops_rate",
		desc:   "metaops (rate)",
		prefix: "metaops",
		unit:   "requests/sec",
		calc:   "derivative",
	},
}

func (m *GpfsCollector) Init(config json.RawMessage) error {
	// Check if already initialized
	if m.init {
		return nil
	}

	m.name = "GpfsCollector"
	if err := m.setup(); err != nil {
		return fmt.Errorf("%s Init(): setup() call failed: %w", m.name, err)
	}
	m.parallel = true

	// Set default mmpmon binary
	m.config.Mmpmon = DEFAULT_GPFS_CMD

	// Read JSON configuration
	if len(config) > 0 {
		err := json.Unmarshal(config, &m.config)
		if err != nil {
			log.Print(err.Error())
			return err
		}
	}
	m.meta = map[string]string{
		"source": m.name,
		"group":  "GPFS",
	}
	m.tags = map[string]string{
		"type":       "node",
		"filesystem": "",
	}
	m.skipFS = make(map[string]struct{})
	for _, fs := range m.config.ExcludeFilesystem {
		m.skipFS[fs] = struct{}{}
	}
	m.lastState = make(map[string]GpfsCollectorState)
	m.lastTimestamp = make(map[string]time.Time)

	// GPFS / IBM Spectrum Scale file system statistics can only be queried by user root
	if !m.config.Sudo {
		user, err := user.Current()
		if err != nil {
			cclog.ComponentError(m.name, "Failed to get current user:", err.Error())
			return err
		}
		if user.Uid != "0" {
			cclog.ComponentError(m.name, "GPFS file system statistics can only be queried by user root")
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

	// when using sudo, the full path of mmpmon must be specified because
	// exec.LookPath will not work as mmpmon is not executable as user
	if m.config.Sudo && !strings.HasPrefix(m.config.Mmpmon, "/") {
		return fmt.Errorf("when using sudo, mmpmon_path must be provided and an absolute path: %s", m.config.Mmpmon)
	}

	// Check if mmpmon is in executable search path
	p, err := exec.LookPath(m.config.Mmpmon)
	if err != nil {
		// if using sudo, exec.lookPath will return EACCES (file mode r-x------), this can be ignored
		if m.config.Sudo && errors.Is(err, syscall.EACCES) {
			cclog.ComponentWarn(m.name, fmt.Sprintf("got error looking for mmpmon binary '%s': %v . This is expected when using sudo, continuing.", m.config.Mmpmon, err))
			// the file was given in the config, use it
			p = m.config.Mmpmon
		} else {
			cclog.ComponentError(m.name, fmt.Sprintf("failed to find mmpmon binary '%s': %v", m.config.Mmpmon, err))
			return fmt.Errorf("failed to find mmpmon binary '%s': %v", m.config.Mmpmon, err)
		}
	}
	m.config.Mmpmon = p

	m.definitions = []GpfsMetricDefinition{}
	if m.config.SendAbsoluteValues {
		for _, def := range GpfsAbsMetrics {
			if !slices.Contains(m.config.ExcludeMetrics, def.name) {
				m.definitions = append(m.definitions, def)
			}
		}
	}
	if m.config.SendDiffValues {
		for _, def := range GpfsDiffMetrics {
			if !slices.Contains(m.config.ExcludeMetrics, def.name) {
				m.definitions = append(m.definitions, def)
			}
		}
	}
	if m.config.SendDerivedValues {
		for _, def := range GpfsDeriveMetrics {
			if !slices.Contains(m.config.ExcludeMetrics, def.name) {
				m.definitions = append(m.definitions, def)
			}
		}
	} else if m.config.SendBandwidths {
		for _, def := range GpfsDeriveMetrics {
			if def.unit == "bytes/sec" {
				if !slices.Contains(m.config.ExcludeMetrics, def.name) {
					m.definitions = append(m.definitions, def)
				}
			}
		}
	}
	if m.config.SendTotalValues {
		for _, def := range GpfsTotalMetrics {
			if !slices.Contains(m.config.ExcludeMetrics, def.name) {
				// only send total metrics of the types requested
				if (def.calc == "none" && m.config.SendAbsoluteValues) ||
					(def.calc == "difference" && m.config.SendDiffValues) ||
					(def.calc == "derivative" && m.config.SendDerivedValues) {
					m.definitions = append(m.definitions, def)
				}
			}
		}
	} else if m.config.SendBandwidths {
		for _, def := range GpfsTotalMetrics {
			if def.unit == "bytes/sec" {
				if !slices.Contains(m.config.ExcludeMetrics, def.name) {
					m.definitions = append(m.definitions, def)
				}
			}
		}
	}
	if len(m.definitions) == 0 {
		return errors.New("no metrics to collect")
	}

	m.init = true
	return nil
}

func (m *GpfsCollector) Read(interval time.Duration, output chan lp.CCMessage) {
	// Check if already initialized
	if !m.init {
		return
	}

	// mmpmon:
	// -p: generate output that can be parsed
	// -s: suppress the prompt on input
	// fs_io_s: Displays I/O statistics per mounted file system
	var cmd *exec.Cmd
	if m.config.Sudo {
		cmd = exec.Command(m.sudoCmd, m.config.Mmpmon, "-p", "-s")
	} else {
		cmd = exec.Command(m.config.Mmpmon, "-p", "-s")
	}

	cmd.Stdin = strings.NewReader("once fs_io_s\n")
	cmdStdout := new(bytes.Buffer)
	cmdStderr := new(bytes.Buffer)
	cmd.Stdout = cmdStdout
	cmd.Stderr = cmdStderr
	err := cmd.Run()
	if err != nil {
		dataStdErr, _ := io.ReadAll(cmdStderr)
		dataStdOut, _ := io.ReadAll(cmdStdout)
		cclog.ComponentError(
			m.name,
			fmt.Sprintf("Read(): Failed to execute command \"%s\": %v\n", cmd.String(), err),
			fmt.Sprintf("Read(): command exit code: \"%d\"\n", cmd.ProcessState.ExitCode()),
			fmt.Sprintf("Read(): command stderr: \"%s\"\n", string(dataStdErr)),
			fmt.Sprintf("Read(): command stdout: \"%s\"\n", string(dataStdOut)),
		)
		return
	}

	// Read I/O statistics
	scanner := bufio.NewScanner(cmdStdout)
	for scanner.Scan() {
		lineSplit := strings.Fields(scanner.Text())

		// Only process lines starting with _fs_io_s_
		if lineSplit[0] != "_fs_io_s_" {
			continue
		}

		key_value := make(map[string]string)
		for i := 1; i < len(lineSplit); i += 2 {
			key_value[lineSplit[i]] = lineSplit[i+1]
		}

		// Ignore keys:
		// _n_:  node IP address,
		// _nn_: node name,
		// _cl_: cluster name,
		// _d_:  number of disks

		filesystem, ok := key_value["_fs_"]
		if !ok {
			cclog.ComponentError(m.name, "Read(): Failed to get filesystem name.")
			continue
		}

		// Skip excluded filesystems
		if _, skip := m.skipFS[filesystem]; skip {
			continue
		}

		// Add filesystem tag
		m.tags["filesystem"] = filesystem

		if _, ok := m.lastState[filesystem]; !ok {
			m.lastState[filesystem] = make(GpfsCollectorState)
		}

		// read the new values from mmpmon
		// return code
		rc, err := strconv.Atoi(key_value["_rc_"])
		if err != nil {
			cclog.ComponentError(m.name, fmt.Sprintf("Read(): Failed to convert return code '%s' to int: %v", key_value["_rc_"], err))
			continue
		}
		if rc != 0 {
			cclog.ComponentError(m.name, fmt.Sprintf("Read(): Filesystem '%s' is not ok.", filesystem))
			continue
		}

		// timestamp
		sec, err := strconv.ParseInt(key_value["_t_"], 10, 64)
		if err != nil {
			cclog.ComponentError(m.name, fmt.Sprintf("Read(): Failed to convert seconds '%s' to int64: %v", key_value["_t_"], err))
			continue
		}
		msec, err := strconv.ParseInt(key_value["_tu_"], 10, 64)
		if err != nil {
			cclog.ComponentError(m.name, fmt.Sprintf("Read(): Failed to convert micro seconds '%s' to int64: %v", key_value["_tu_"], err))
			continue
		}
		timestamp := time.Unix(sec, msec*1000)

		// time difference to last time stamp
		var timeDiff float64 = 0
		if lastTime, ok := m.lastTimestamp[filesystem]; !ok {
			m.lastTimestamp[filesystem] = time.Time{}
		} else {
			timeDiff = timestamp.Sub(lastTime).Seconds()
		}

		// get values of all abs metrics
		newstate := make(GpfsCollectorState)
		for _, metric := range GpfsAbsMetrics {
			value, err := strconv.ParseInt(key_value[metric.prefix], 10, 64)
			if err != nil {
				cclog.ComponentError(m.name, fmt.Sprintf("Read(): Failed to convert %s '%s' to int64: %v", metric.desc, key_value[metric.prefix], err))
				continue
			}
			newstate[metric.prefix] = value
		}

		// compute total metrics (map[...] will return 0 if key not found)
		// bytes read and written
		if br, br_ok := newstate["_br_"]; br_ok {
			newstate["bytesTotal"] = newstate["bytesTotal"] + br
		}
		if bw, bw_ok := newstate["_bw_"]; bw_ok {
			newstate["bytesTotal"] = newstate["bytesTotal"] + bw
		}
		// read and write count
		if rdc, rdc_ok := newstate["_rdc_"]; rdc_ok {
			newstate["iops"] = newstate["iops"] + rdc
		}
		if wc, wc_ok := newstate["_wc_"]; wc_ok {
			newstate["iops"] = newstate["iops"] + wc
		}
		// meta operations
		if oc, oc_ok := newstate["_oc_"]; oc_ok {
			newstate["metaops"] = newstate["metaops"] + oc
		}
		if cc, cc_ok := newstate["_cc_"]; cc_ok {
			newstate["metaops"] = newstate["metaops"] + cc
		}
		if dir, dir_ok := newstate["_dir_"]; dir_ok {
			newstate["metaops"] = newstate["metaops"] + dir
		}
		if iu, iu_ok := newstate["_iu_"]; iu_ok {
			newstate["metaops"] = newstate["metaops"] + iu
		}
		// send desired metrics for this filesystem
		for _, metric := range m.definitions {
			vold, vold_ok := m.lastState[filesystem][metric.prefix]
			vnew, vnew_ok := newstate[metric.prefix]
			var value interface{}
			value_ok := false
			switch metric.calc {
			case "none":
				if vnew_ok {
					value = vnew
					value_ok = true
				} else if vold_ok {
					// for absolute values, if the new value is not available, report no change
					value = vold
					value_ok = true
				}
			case "difference":
				if vnew_ok && vold_ok {
					value = vnew - vold
					if value.(int64) < 0 {
						value = 0
					}
					value_ok = true
				} else if vold_ok {
					// if the difference is not computable, return 0
					value = 0
					value_ok = true
				}
			case "derivative":
				if vnew_ok && vold_ok && timeDiff > 0 {
					value = float64(vnew-vold) / timeDiff
					if value.(float64) < 0 {
						value = 0
					}
					value_ok = true
				} else if vold_ok {
					// if the difference is not computable, return 0
					value = 0
					value_ok = true
				}
			}
			if value_ok {
				y, err := lp.NewMetric(metric.name, m.tags, m.meta, value, timestamp)
				if err == nil {
					if len(metric.unit) > 0 {
						y.AddMeta("unit", metric.unit)
					}
					output <- y
				}
			} else {
				// the value could not be computed correctly
				cclog.ComponentWarn(m.name, fmt.Sprintf("Read(): Could not compute value for filesystem %s of metric %s: vold_ok = %t, vnew_ok = %t", filesystem, metric.name, vold_ok, vnew_ok))
			}
		}

		// Save new state, if it contains proper values
		if len(newstate) > 0 {
			m.lastState[filesystem] = newstate
			m.lastTimestamp[filesystem] = timestamp
		}
	}
}

func (m *GpfsCollector) Close() {
	m.init = false
}
