package collectors

import (
	"fmt"
	"io/ioutil"
	"os"

	cclog "github.com/ClusterCockpit/cc-metric-collector/internal/ccLogger"
	lp "github.com/ClusterCockpit/cc-metric-collector/internal/ccMetric"
	"golang.org/x/sys/unix"

	"encoding/json"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const IB_BASEPATH = `/sys/class/infiniband/`

type InfinibandCollectorInfo struct {
	LID              string            // IB local Identifier (LID)
	device           string            // IB device
	port             string            // IB device port
	portCounterFiles map[string]string // mapping counter name -> sysfs file
	tagSet           map[string]string // corresponding tag list
	stats            map[string]int64
}

type InfinibandCollector struct {
	metricCollector
	config struct {
		ExcludeDevices     []string `json:"exclude_devices,omitempty"` // IB device to exclude e.g. mlx5_0
		SendAbsoluteValues bool     `json:"send_abs_values"`
		SendDerivedValues  bool     `json:"send_derived_values"`
	}
	info          []*InfinibandCollectorInfo
	lastTimestamp time.Time // Store time stamp of last tick to derive bandwidths
}

// Init initializes the Infiniband collector by walking through files below IB_BASEPATH
func (m *InfinibandCollector) Init(config json.RawMessage) error {

	// Check if already initialized
	if m.init {
		return nil
	}

	var err error
	m.name = "InfinibandCollector"
	m.setup()
	m.meta = map[string]string{
		"source": m.name,
		"group":  "Network",
	}
	m.lastTimestamp = time.Now()
	m.config.SendAbsoluteValues = true
	m.config.SendDerivedValues = false
	if len(config) > 0 {
		err = json.Unmarshal(config, &m.config)
		if err != nil {
			return err
		}
	}

	// Loop for all InfiniBand directories
	globPattern := filepath.Join(IB_BASEPATH, "*", "ports", "*")
	ibDirs, err := filepath.Glob(globPattern)
	if err != nil {
		return fmt.Errorf("unable to glob files with pattern %s: %v", globPattern, err)
	}
	if ibDirs == nil {
		return fmt.Errorf("unable to find any directories with pattern %s", globPattern)
	}

	for _, path := range ibDirs {

		// Skip, when no LID is assigned
		line, err := ioutil.ReadFile(filepath.Join(path, "lid"))
		if err != nil {
			continue
		}
		LID := strings.TrimSpace(string(line))
		if LID == "0x0" {
			continue
		}

		// Get device and port component
		pathSplit := strings.Split(path, string(os.PathSeparator))
		device := pathSplit[4]
		port := pathSplit[6]

		// Skip excluded devices
		skip := false
		for _, excludedDevice := range m.config.ExcludeDevices {
			if excludedDevice == device {
				skip = true
				break
			}
		}
		if skip {
			continue
		}

		// Check access to counter files
		countersDir := filepath.Join(path, "counters")
		portCounterFiles := map[string]string{
			"ib_recv":      filepath.Join(countersDir, "port_rcv_data"),
			"ib_xmit":      filepath.Join(countersDir, "port_xmit_data"),
			"ib_recv_pkts": filepath.Join(countersDir, "port_rcv_packets"),
			"ib_xmit_pkts": filepath.Join(countersDir, "port_xmit_packets"),
		}
		for _, counterFile := range portCounterFiles {
			err := unix.Access(counterFile, unix.R_OK)
			if err != nil {
				return fmt.Errorf("unable to access %s: %v", counterFile, err)
			}
		}

		m.info = append(m.info,
			&InfinibandCollectorInfo{
				LID:              LID,
				device:           device,
				port:             port,
				portCounterFiles: portCounterFiles,
				tagSet: map[string]string{
					"type":   "node",
					"device": device,
					"port":   port,
					"lid":    LID,
				},
				stats: map[string]int64{
					"ib_recv":      0,
					"ib_xmit":      0,
					"ib_recv_pkts": 0,
					"ib_xmit_pkts": 0,
				},
			})
	}

	if len(m.info) == 0 {
		return fmt.Errorf("found no IB devices")
	}

	m.init = true
	return nil
}

// Read reads Infiniband counter files below IB_BASEPATH
func (m *InfinibandCollector) Read(interval time.Duration, output chan lp.CCMetric) {

	// Check if already initialized
	if !m.init {
		return
	}

	now := time.Now()
	tdiff := now.Sub(m.lastTimestamp)
	for _, info := range m.info {
		for counterName, counterFile := range info.portCounterFiles {
			line, err := ioutil.ReadFile(counterFile)
			if err != nil {
				cclog.ComponentError(
					m.name,
					fmt.Sprintf("Read(): Failed to read from file '%s': %v", counterFile, err))
				continue
			}
			data := strings.TrimSpace(string(line))
			v, err := strconv.ParseInt(data, 10, 64)
			if err != nil {
				cclog.ComponentError(
					m.name,
					fmt.Sprintf("Read(): Failed to convert Infininiband metrice %s='%s' to int64: %v", counterName, data, err))
				continue
			}
			if m.config.SendAbsoluteValues {
				if y, err := lp.New(counterName, info.tagSet, m.meta, map[string]interface{}{"value": v}, now); err == nil {
					output <- y
				}
			}
			if m.config.SendDerivedValues {
				diff := float64((v - info.stats[counterName])) / tdiff.Seconds()
				if y, err := lp.New(counterName+"_bw", info.tagSet, m.meta, map[string]interface{}{"value": diff}, now); err == nil {
					output <- y
				}
				info.stats[counterName] = v
			}
		}

	}
	m.lastTimestamp = now
}

func (m *InfinibandCollector) Close() {
	m.init = false
}
