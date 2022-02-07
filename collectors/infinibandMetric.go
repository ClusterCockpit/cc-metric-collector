package collectors

import (
	"fmt"
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
}

type InfinibandCollector struct {
	metricCollector
	config struct {
		ExcludeDevices []string `json:"exclude_devices,omitempty"` // IB device to exclude e.g. mlx5_0
	}
	info []InfinibandCollectorInfo
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
		return fmt.Errorf("Unable to glob files with pattern %s: %v", globPattern, err)
	}
	if ibDirs == nil {
		return fmt.Errorf("Unable to find any directories with pattern %s", globPattern)
	}

	for _, path := range ibDirs {

		// Skip, when no LID is assigned
		LID, ok := readOneLine(path + "/lid")
		if !ok || LID == "0x0" {
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
				return fmt.Errorf("Unable to access %s: %v", counterFile, err)
			}
		}

		m.info = append(m.info,
			InfinibandCollectorInfo{
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
			})
	}

	if len(m.info) == 0 {
		return fmt.Errorf("Found no IB devices")
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
	for i := range m.info {

		// device info
		info := &m.info[i]
		for counterName, counterFile := range info.portCounterFiles {
			data, ok := readOneLine(counterFile)
			if !ok {
				cclog.ComponentError(
					m.name,
					fmt.Sprintf("Read(): Failed to read one line from file '%s'", counterFile))
				continue
			}
			v, err := strconv.ParseInt(data, 10, 64)
			if err != nil {
				cclog.ComponentError(
					m.name,
					fmt.Sprintf("Read(): Failed to convert Infininiband metrice %s='%s' to int64: %v", counterName, data, err))
				continue
			}
			if y, err := lp.New(counterName, info.tagSet, m.meta, map[string]interface{}{"value": v}, now); err == nil {
				output <- y
			}
		}

	}
}

func (m *InfinibandCollector) Close() {
	m.init = false
}
