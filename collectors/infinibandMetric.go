package collectors

import (
	"fmt"
	"os"

	cclog "github.com/ClusterCockpit/cc-metric-collector/pkg/ccLogger"
	lp "github.com/ClusterCockpit/cc-metric-collector/pkg/ccMetric"
	"golang.org/x/sys/unix"

	"encoding/json"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const IB_BASEPATH = "/sys/class/infiniband/"

type InfinibandCollectorMetric struct {
	path  string
	unit  string
	scale int64
}

type InfinibandCollectorInfo struct {
	LID              string                               // IB local Identifier (LID)
	device           string                               // IB device
	port             string                               // IB device port
	portCounterFiles map[string]InfinibandCollectorMetric // mapping counter name -> InfinibandCollectorMetric
	tagSet           map[string]string                    // corresponding tag list
	lastState        map[string]int64                     // State from last measurement
}

type InfinibandCollector struct {
	metricCollector
	config struct {
		ExcludeDevices     []string `json:"exclude_devices,omitempty"` // IB device to exclude e.g. mlx5_0
		SendAbsoluteValues bool     `json:"send_abs_values"`           // Send absolut values as read from sys filesystem
		SendDerivedValues  bool     `json:"send_derived_values"`       // Send derived values e.g. rates
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
	m.parallel = true
	m.meta = map[string]string{
		"source": m.name,
		"group":  "Network",
	}

	// Set default configuration,
	m.config.SendAbsoluteValues = true
	m.config.SendDerivedValues = false
	// Read configuration file, allow overwriting default config
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
		line, err := os.ReadFile(filepath.Join(path, "lid"))
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
		portCounterFiles := map[string]InfinibandCollectorMetric{
			"ib_recv":      {path: filepath.Join(countersDir, "port_rcv_data"), unit: "bytes", scale: 4},
			"ib_xmit":      {path: filepath.Join(countersDir, "port_xmit_data"), unit: "bytes", scale: 4},
			"ib_recv_pkts": {path: filepath.Join(countersDir, "port_rcv_packets"), unit: "packets", scale: 1},
			"ib_xmit_pkts": {path: filepath.Join(countersDir, "port_xmit_packets"), unit: "packets", scale: 1},
		}
		for _, counter := range portCounterFiles {
			err := unix.Access(counter.path, unix.R_OK)
			if err != nil {
				return fmt.Errorf("unable to access %s: %v", counter.path, err)
			}
		}

		// Initialize last state
		lastState := make(map[string]int64)
		for counter := range portCounterFiles {
			lastState[counter] = -1
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
				lastState: lastState,
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

	// Current time stamp
	now := time.Now()
	// time difference to last time stamp
	timeDiff := now.Sub(m.lastTimestamp).Seconds()
	// Save current timestamp
	m.lastTimestamp = now

	for _, info := range m.info {
		for counterName, counterDef := range info.portCounterFiles {

			// Read counter file
			line, err := os.ReadFile(counterDef.path)
			if err != nil {
				cclog.ComponentError(
					m.name,
					fmt.Sprintf("Read(): Failed to read from file '%s': %v", counterDef.path, err))
				continue
			}
			data := strings.TrimSpace(string(line))

			// convert counter to int64
			v, err := strconv.ParseInt(data, 10, 64)
			if err != nil {
				cclog.ComponentError(
					m.name,
					fmt.Sprintf("Read(): Failed to convert Infininiband metrice %s='%s' to int64: %v", counterName, data, err))
				continue
			}
			// Scale raw value
			v *= counterDef.scale

			// Send absolut values
			if m.config.SendAbsoluteValues {
				if y, err := lp.New(counterName, info.tagSet, m.meta, map[string]interface{}{"value": v}, now); err == nil {
					y.AddMeta("unit", counterDef.unit)
					output <- y
				}
			}

			// Send derived values
			if m.config.SendDerivedValues {
				if info.lastState[counterName] >= 0 {
					rate := float64((v - info.lastState[counterName])) / timeDiff
					if y, err := lp.New(counterName+"_bw", info.tagSet, m.meta, map[string]interface{}{"value": rate}, now); err == nil {
						y.AddMeta("unit", counterDef.unit+"/sec")
						output <- y
					}
				}
				// Save current state
				info.lastState[counterName] = v
			}
		}

	}
}

func (m *InfinibandCollector) Close() {
	m.init = false
}
