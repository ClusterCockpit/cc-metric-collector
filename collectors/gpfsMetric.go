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
	"fmt"
	"io"
	"log"
	"os/exec"
	"os/user"
	"strconv"
	"strings"
	"time"

	cclog "github.com/ClusterCockpit/cc-lib/ccLogger"
	lp "github.com/ClusterCockpit/cc-lib/ccMessage"
)

const DEFAULT_GPFS_CMD = "mmpmon"

type GpfsCollectorLastState struct {
	numOpens    	int64
	numCloses 		int64
	numReads		int64
	numWrites		int64
	numReaddirs		int64
	numInodeUpdates int64
	bytesRead    	int64
	bytesWritten 	int64
	bytesTotal		int64
	iops			int64
	metaops			int64
}

type GpfsCollector struct {
	metricCollector
	tags   map[string]string
	config struct {
		Mmpmon            string   `json:"mmpmon_path,omitempty"`
		ExcludeFilesystem []string `json:"exclude_filesystem,omitempty"`
		SendBandwidths    bool     `json:"send_bandwidths"`
		SendTotalValues   bool     `json:"send_total_values"`
		SendDerivedValues bool     `json:"send_derived_values"`
	}
	skipFS        map[string]struct{}
	lastTimestamp time.Time // Store time stamp of last tick to derive bandwidths
	lastState     map[string]GpfsCollectorLastState
}

func (m *GpfsCollector) Init(config json.RawMessage) error {
	// Check if already initialized
	if m.init {
		return nil
	}

	var err error
	m.name = "GpfsCollector"
	m.setup()
	m.parallel = true

	// Set default mmpmon binary
	m.config.Mmpmon = DEFAULT_GPFS_CMD

	// Read JSON configuration
	if len(config) > 0 {
		err = json.Unmarshal(config, &m.config)
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
	m.lastState = make(map[string]GpfsCollectorLastState)

	// GPFS / IBM Spectrum Scale file system statistics can only be queried by user root
	user, err := user.Current()
	if err != nil {
		return fmt.Errorf("failed to get current user: %v", err)
	}
	if user.Uid != "0" {
		return fmt.Errorf("GPFS file system statistics can only be queried by user root")
	}

	// Check if mmpmon is in executable search path
	p, err := exec.LookPath(m.config.Mmpmon)
	if err != nil {
		return fmt.Errorf("failed to find mmpmon binary '%s': %v", m.config.Mmpmon, err)
	}
	m.config.Mmpmon = p

	m.init = true
	return nil
}

func (m *GpfsCollector) Read(interval time.Duration, output chan lp.CCMessage) {
	// Check if already initialized
	if !m.init {
		return
	}

	// Current time stamp
	now := time.Now()
	// time difference to last time stamp
	timeDiff := now.Sub(m.lastTimestamp).Seconds()
	// Save current timestamp
	m.lastTimestamp = now

	// mmpmon:
	// -p: generate output that can be parsed
	// -s: suppress the prompt on input
	// fs_io_s: Displays I/O statistics per mounted file system
	cmd := exec.Command(m.config.Mmpmon, "-p", "-s")
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
			cclog.ComponentError(
				m.name,
				"Read(): Failed to get filesystem name.")
			continue
		}

		// Skip excluded filesystems
		if _, skip := m.skipFS[filesystem]; skip {
			continue
		}

		// Add filesystem tag
		m.tags["filesystem"] = filesystem

		// Create initial last state
		if m.config.SendBandwidths {
			if _, ok := m.lastState[filesystem]; !ok {
				m.lastState[filesystem] = GpfsCollectorLastState{
					bytesRead:    -1,
					bytesWritten: -1,
				}
			}
		}

		if m.config.SendDerivedValues {
			if _, ok := m.lastState[filesystem]; !ok {
				m.lastState[filesystem] = GpfsCollectorLastState{
					numReads:    -1,
					numWrites: -1,
					numOpens: -1,
					numCloses: -1,
					numReaddirs: -1,
					numInodeUpdates: -1,
					bytesTotal: -1,
					iops: -1,
					metaops: -1,
				}
			}		
		}

		// return code
		rc, err := strconv.Atoi(key_value["_rc_"])
		if err != nil {
			cclog.ComponentError(
				m.name,
				fmt.Sprintf("Read(): Failed to convert return code '%s' to int: %v", key_value["_rc_"], err))
			continue
		}
		if rc != 0 {
			cclog.ComponentError(
				m.name,
				fmt.Sprintf("Read(): Filesystem '%s' is not ok.", filesystem))
			continue
		}

		sec, err := strconv.ParseInt(key_value["_t_"], 10, 64)
		if err != nil {
			cclog.ComponentError(
				m.name,
				fmt.Sprintf("Read(): Failed to convert seconds '%s' to int64: %v", key_value["_t_"], err))
			continue
		}
		msec, err := strconv.ParseInt(key_value["_tu_"], 10, 64)
		if err != nil {
			cclog.ComponentError(
				m.name,
				fmt.Sprintf("Read(): Failed to convert micro seconds '%s' to int64: %v", key_value["_tu_"], err))
			continue
		}
		timestamp := time.Unix(sec, msec*1000)

		// bytes read
		bytesRead, err := strconv.ParseInt(key_value["_br_"], 10, 64)
		if err != nil {
			cclog.ComponentError(
				m.name,
				fmt.Sprintf("Read(): Failed to convert bytes read '%s' to int64: %v", key_value["_br_"], err))
			continue
		}
		if y, err :=
			lp.NewMessage(
				"gpfs_bytes_read",
				m.tags,
				m.meta,
				map[string]interface{}{
					"value": bytesRead,
				},
				timestamp,
			); err == nil {
			y.AddMeta("unit", "bytes")
			output <- y
		}
		if m.config.SendBandwidths {
			if lastBytesRead := m.lastState[filesystem].bytesRead; lastBytesRead >= 0 {
				bwRead := float64(bytesRead-lastBytesRead) / timeDiff
				if y, err :=
					lp.NewMessage(
						"gpfs_bw_read",
						m.tags,
						m.meta,
						map[string]interface{}{
							"value": bwRead,
						},
						timestamp,
					); err == nil {
					y.AddMeta("unit", "bytes/sec")
					output <- y
				}
			}
		}

		// bytes written
		bytesWritten, err := strconv.ParseInt(key_value["_bw_"], 10, 64)
		if err != nil {
			cclog.ComponentError(
				m.name,
				fmt.Sprintf("Read(): Failed to convert bytes written '%s' to int64: %v", key_value["_bw_"], err))
			continue
		}
		if y, err :=
			lp.NewMessage(
				"gpfs_bytes_written",
				m.tags,
				m.meta,
				map[string]interface{}{
					"value": bytesWritten,
				},
				timestamp,
			); err == nil {
			y.AddMeta("unit", "bytes")
			output <- y
		}
		if m.config.SendBandwidths {
			if lastBytesWritten := m.lastState[filesystem].bytesWritten; lastBytesWritten >= 0 {
				bwWrite := float64(bytesWritten-lastBytesWritten) / timeDiff
				if y, err :=
					lp.NewMessage(
						"gpfs_bw_write",
						m.tags,
						m.meta,
						map[string]interface{}{
							"value": bwWrite,
						},
						timestamp,
					); err == nil {
					y.AddMeta("unit", "bytes/sec")
					output <- y
				}
			}
		}

		// number of opens
		numOpens, err := strconv.ParseInt(key_value["_oc_"], 10, 64)
		if err != nil {
			cclog.ComponentError(
				m.name,
				fmt.Sprintf("Read(): Failed to convert number of opens '%s' to int64: %v", key_value["_oc_"], err))
			continue
		}
		if y, err := lp.NewMessage("gpfs_num_opens", m.tags, m.meta, map[string]interface{}{"value": numOpens}, timestamp); err == nil {
			output <- y
		}
		if m.config.SendDerivedValues {
			if lastNumOpens := m.lastState[filesystem].numOpens; lastNumOpens >= 0 {
				opensRate := float64(numOpens-lastNumOpens) / timeDiff
				if y, err :=
					lp.NewMessage(
						"gpfs_opens_rate",
						m.tags,
						m.meta,
						map[string]interface{}{
							"value": opensRate,
						},
						timestamp,
					); err == nil {
					y.AddMeta("unit", "requests/sec")
					output <- y
				}
			}
		}

		// number of closes
		numCloses, err := strconv.ParseInt(key_value["_cc_"], 10, 64)
		if err != nil {
			cclog.ComponentError(
				m.name,
				fmt.Sprintf("Read(): Failed to convert number of closes: '%s' to int64: %v", key_value["_cc_"], err))
			continue
		}
		if y, err := lp.NewMessage("gpfs_num_closes", m.tags, m.meta, map[string]interface{}{"value": numCloses}, timestamp); err == nil {
			output <- y
		}
		if m.config.SendDerivedValues {
			if lastNumCloses := m.lastState[filesystem].numCloses; lastNumCloses >= 0 {
				closesRate := float64(numCloses-lastNumCloses) / timeDiff
				if y, err :=
					lp.NewMessage(
						"gpfs_closes_rate",
						m.tags,
						m.meta,
						map[string]interface{}{
							"value": closesRate,
						},
						timestamp,
					); err == nil {
					y.AddMeta("unit", "requests/sec")
					output <- y
				}
			}
		}

		// number of reads
		numReads, err := strconv.ParseInt(key_value["_rdc_"], 10, 64)
		if err != nil {
			cclog.ComponentError(
				m.name,
				fmt.Sprintf("Read(): Failed to convert number of reads: '%s' to int64: %v", key_value["_rdc_"], err))
			continue
		}
		if y, err := lp.NewMessage("gpfs_num_reads", m.tags, m.meta, map[string]interface{}{"value": numReads}, timestamp); err == nil {
			output <- y
		}
		if m.config.SendDerivedValues {
			if lastNumReads := m.lastState[filesystem].numReads; lastNumReads >= 0 {
				readsRate := float64(numReads-lastNumReads) / timeDiff
				if y, err :=
					lp.NewMessage(
						"gpfs_reads_rate",
						m.tags,
						m.meta,
						map[string]interface{}{
							"value": readsRate,
						},
						timestamp,
					); err == nil {
					y.AddMeta("unit", "requests/sec")
					output <- y
				}
			}
		}

		// number of writes
		numWrites, err := strconv.ParseInt(key_value["_wc_"], 10, 64)
		if err != nil {
			cclog.ComponentError(
				m.name,
				fmt.Sprintf("Read(): Failed to convert number of writes: '%s' to int64: %v", key_value["_wc_"], err))
			continue
		}
		if y, err := lp.NewMessage("gpfs_num_writes", m.tags, m.meta, map[string]interface{}{"value": numWrites}, timestamp); err == nil {
			output <- y
		}
		if m.config.SendDerivedValues {
			if lastNumWrites := m.lastState[filesystem].numWrites; lastNumWrites >= 0 {
				writesRate := float64(numWrites-lastNumWrites) / timeDiff
				if y, err :=
					lp.NewMessage(
						"gpfs_writes_rate",
						m.tags,
						m.meta,
						map[string]interface{}{
							"value": writesRate,
						},
						timestamp,
					); err == nil {
					y.AddMeta("unit", "requests/sec")
					output <- y
				}
			}
		}

		// number of read directories
		numReaddirs, err := strconv.ParseInt(key_value["_dir_"], 10, 64)
		if err != nil {
			cclog.ComponentError(
				m.name,
				fmt.Sprintf("Read(): Failed to convert number of read directories: '%s' to int64: %v", key_value["_dir_"], err))
			continue
		}
		if y, err := lp.NewMessage("gpfs_num_readdirs", m.tags, m.meta, map[string]interface{}{"value": numReaddirs}, timestamp); err == nil {
			output <- y
		}
		if m.config.SendDerivedValues {
			if lastNumReaddirs := m.lastState[filesystem].numReaddirs; lastNumReaddirs >= 0 {
				readdirsRate := float64(numReaddirs-lastNumReaddirs) / timeDiff
				if y, err :=
					lp.NewMessage(
						"gpfs_readdirs_rate",
						m.tags,
						m.meta,
						map[string]interface{}{
							"value": readdirsRate,
						},
						timestamp,
					); err == nil {
					y.AddMeta("unit", "requests/sec")
					output <- y
				}
			}
		}

		// Number of inode updates
		numInodeUpdates, err := strconv.ParseInt(key_value["_iu_"], 10, 64)
		if err != nil {
			cclog.ComponentError(
				m.name,
				fmt.Sprintf("Read(): Failed to convert number of inode updates: '%s' to int: %v", key_value["_iu_"], err))
			continue
		}
		if y, err := lp.NewMessage("gpfs_num_inode_updates", m.tags, m.meta, map[string]interface{}{"value": numInodeUpdates}, timestamp); err == nil {
			output <- y
		}
		if m.config.SendDerivedValues {
			if lastNumInodeUpdates := m.lastState[filesystem].numInodeUpdates; lastNumInodeUpdates >= 0 {
				inodeUpdatesRate := float64(numInodeUpdates-lastNumInodeUpdates) / timeDiff
				if y, err :=
					lp.NewMessage(
						"gpfs_inode_updates_rate",
						m.tags,
						m.meta,
						map[string]interface{}{
							"value": inodeUpdatesRate,
						},
						timestamp,
					); err == nil {
					y.AddMeta("unit", "requests/sec")
					output <- y
				}
			}
		}

		// Total values
		bytesTotal := int64(-1);
		iops := int64(-1);
		metaops := int64(-1);
		if m.config.SendTotalValues {
			bytesTotal = bytesRead + bytesWritten
			if y, err :=
				lp.NewMessage("gpfs_bytes_total",
					m.tags,
					m.meta,
					map[string]interface{}{
						"value": bytesTotal,
					},
					timestamp,
				); err == nil {
				y.AddMeta("unit", "bytes")
				output <- y
			}
			if m.config.SendBandwidths {
				if lastBytesTotal := m.lastState[filesystem].bytesTotal; lastBytesTotal >= 0 {
					bwTotal := float64(bytesTotal-lastBytesTotal) / timeDiff
					if y, err :=
						lp.NewMessage(
							"gpfs_bw_total",
							m.tags,
							m.meta,
							map[string]interface{}{
								"value": bwTotal,
							},
							timestamp,
						); err == nil {
						y.AddMeta("unit", "bytes/sec")
						output <- y
					}
				}
			}

			iops = numReads + numWrites
			if y, err :=
				lp.NewMessage("gpfs_iops",
					m.tags,
					m.meta,
					map[string]interface{}{
						"value": iops,
					},
					timestamp,
				); err == nil {
				output <- y
			}
			if m.config.SendDerivedValues {
				if lastIops := m.lastState[filesystem].iops; lastIops >= 0 {
					iopsRate := float64(iops-lastIops) / timeDiff
					if y, err :=
						lp.NewMessage(
							"gpfs_iops_rate",
							m.tags,
							m.meta,
							map[string]interface{}{
								"value": iopsRate,
							},
							timestamp,
						); err == nil {
						y.AddMeta("unit", "requests/sec")
						output <- y
					}
				}
			}
	
			metaops = numInodeUpdates + numCloses + numOpens + numReaddirs
			if y, err :=
				lp.NewMessage("gpfs_metaops",
					m.tags,
					m.meta,
					map[string]interface{}{
						"value": metaops,
					},
					timestamp,
				); err == nil {
				output <- y
			}
			if m.config.SendDerivedValues {
				if lastMetaops := m.lastState[filesystem].metaops; lastMetaops >= 0 {
					metaopsRate := float64(metaops-lastMetaops) / timeDiff
					if y, err :=
						lp.NewMessage(
							"gpfs_metaops_rate",
							m.tags,
							m.meta,
							map[string]interface{}{
								"value": metaopsRate,
							},
							timestamp,
						); err == nil {
						y.AddMeta("unit", "requests/sec")
						output <- y
					}
				}
			}
		}

		// Save last state
		m.lastState[filesystem] = GpfsCollectorLastState{
			bytesRead:    bytesRead,
			bytesWritten: bytesWritten,
			numOpens: numOpens,
			numCloses: numCloses,
			numReads: numReads,
			numWrites: numWrites,
			numReaddirs: numReaddirs,
			numInodeUpdates: numInodeUpdates,
			bytesTotal: bytesTotal,
			iops: iops,
			metaops: metaops,
		}

	}
}

func (m *GpfsCollector) Close() {
	m.init = false
}
