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
	"slices"
	"time"

	cclog "github.com/ClusterCockpit/cc-lib/v2/ccLogger"
	lp "github.com/ClusterCockpit/cc-lib/v2/ccMessage"
	"github.com/ClusterCockpit/go-rocm-smi/pkg/rocm_smi"
)

type RocmSmiCollectorConfig struct {
	ExcludeMetrics     []string `json:"exclude_metrics,omitempty"`
	ExcludeDevices     []string `json:"exclude_devices,omitempty"`
	AddPciInfoTag      bool     `json:"add_pci_info_tag,omitempty"`
	UsePciInfoAsTypeId bool     `json:"use_pci_info_as_type_id,omitempty"`
	AddSerialMeta      bool     `json:"add_serial_meta,omitempty"`
}

type RocmSmiCollectorDevice struct {
	device         rocm_smi.DeviceHandle
	index          int
	tags           map[string]string // default tags
	meta           map[string]string // default meta information
	excludeMetrics map[string]bool   // copy of exclude metrics from config
}

type RocmSmiCollector struct {
	metricCollector
	config  RocmSmiCollectorConfig // the configuration structure
	devices []RocmSmiCollectorDevice
}

// Functions to implement MetricCollector interface
// Init(...), Read(...), Close()
// See: metricCollector.go

// Init initializes the sample collector
// Called once by the collector manager
// All tags, meta data tags and metrics that do not change over the runtime should be set here
func (m *RocmSmiCollector) Init(config json.RawMessage) error {
	var err error = nil
	// Always set the name early in Init() to use it in cclog.Component* functions
	m.name = "RocmSmiCollector"
	// This is for later use, also call it early
	if err := m.setup(); err != nil {
		return fmt.Errorf("%s Init(): setup() call failed: %w", m.name, err)
	}
	// Define meta information sent with each metric
	// (Can also be dynamic or this is the basic set with extension through AddMeta())
	//m.meta = map[string]string{"source": m.name, "group": "AMD"}
	// Define tags sent with each metric
	// The 'type' tag is always needed, it defines the granulatity of the metric
	// node -> whole system
	// socket -> CPU socket (requires socket ID as 'type-id' tag)
	// cpu -> single CPU hardware thread (requires cpu ID as 'type-id' tag)
	//m.tags = map[string]string{"type": "node"}
	// Read in the JSON configuration
	if len(config) > 0 {
		err = json.Unmarshal(config, &m.config)
		if err != nil {
			cclog.ComponentError(m.name, "Error reading config:", err.Error())
			return err
		}
	}

	ret := rocm_smi.Init()
	if ret != rocm_smi.STATUS_SUCCESS {
		err = errors.New("failed to initialize ROCm SMI library")
		cclog.ComponentError(m.name, err.Error())
		return err
	}

	numDevs, ret := rocm_smi.NumMonitorDevices()
	if ret != rocm_smi.STATUS_SUCCESS {
		err = errors.New("failed to get number of GPUs from ROCm SMI library")
		cclog.ComponentError(m.name, err.Error())
		return err
	}

	m.devices = make([]RocmSmiCollectorDevice, 0)

	for i := range numDevs {
		str_i := fmt.Sprintf("%d", i)
		if slices.Contains(m.config.ExcludeDevices, str_i) {
			continue
		}
		device, ret := rocm_smi.DeviceGetHandleByIndex(i)
		if ret != rocm_smi.STATUS_SUCCESS {
			err = fmt.Errorf("failed to get handle for GPU %d", i)
			cclog.ComponentError(m.name, err.Error())
			return err
		}

		pciInfo, ret := rocm_smi.DeviceGetPciInfo(device)
		if ret != rocm_smi.STATUS_SUCCESS {
			err = fmt.Errorf("failed to get PCI information for GPU %d", i)
			cclog.ComponentError(m.name, err.Error())
			return err
		}

		pciId := fmt.Sprintf(
			"%08X:%02X:%02X.%X",
			pciInfo.Domain,
			pciInfo.Bus,
			pciInfo.Device,
			pciInfo.Function)

		if slices.Contains(m.config.ExcludeDevices, pciId) {
			continue
		}

		dev := RocmSmiCollectorDevice{
			device: device,
			tags: map[string]string{
				"type":    "accelerator",
				"type-id": str_i,
			},
			meta: map[string]string{
				"source": m.name,
				"group":  "AMD",
			},
		}
		if m.config.UsePciInfoAsTypeId {
			dev.tags["type-id"] = pciId
		} else if m.config.AddPciInfoTag {
			dev.tags["pci_identifier"] = pciId
		}

		if m.config.AddSerialMeta {
			serial, ret := rocm_smi.DeviceGetSerialNumber(device)
			if ret != rocm_smi.STATUS_SUCCESS {
				cclog.ComponentError(m.name, "Unable to get serial number for device at index", i, ":", rocm_smi.StatusStringNoError(ret))
			} else {
				dev.meta["serial"] = serial
			}
		}
		// Add excluded metrics
		dev.excludeMetrics = map[string]bool{}
		for _, e := range m.config.ExcludeMetrics {
			dev.excludeMetrics[e] = true
		}
		dev.index = i
		m.devices = append(m.devices, dev)
	}

	// Set this flag only if everything is initialized properly, all required files exist, ...
	m.init = true
	return err
}

// Read collects all metrics belonging to the sample collector
// and sends them through the output channel to the collector manager
func (m *RocmSmiCollector) Read(interval time.Duration, output chan lp.CCMessage) {
	// Create a sample metric
	timestamp := time.Now()

	for _, dev := range m.devices {
		metrics, ret := rocm_smi.DeviceGetMetrics(dev.device)
		if ret != rocm_smi.STATUS_SUCCESS {
			cclog.ComponentError(m.name, "Unable to get metrics for device at index", dev.index, ":", rocm_smi.StatusStringNoError(ret))
			continue
		}

		if !dev.excludeMetrics["rocm_gfx_util"] {
			value := metrics.Average_gfx_activity
			y, err := lp.NewMessage("rocm_gfx_util", dev.tags, dev.meta, map[string]any{"value": value}, timestamp)
			if err == nil {
				output <- y
			}
		}
		if !dev.excludeMetrics["rocm_umc_util"] {
			value := metrics.Average_umc_activity
			y, err := lp.NewMessage("rocm_umc_util", dev.tags, dev.meta, map[string]any{"value": value}, timestamp)
			if err == nil {
				output <- y
			}
		}
		if !dev.excludeMetrics["rocm_mm_util"] {
			value := metrics.Average_mm_activity
			y, err := lp.NewMessage("rocm_mm_util", dev.tags, dev.meta, map[string]any{"value": value}, timestamp)
			if err == nil {
				output <- y
			}
		}
		if !dev.excludeMetrics["rocm_avg_power"] {
			value := metrics.Average_socket_power
			y, err := lp.NewMessage("rocm_avg_power", dev.tags, dev.meta, map[string]any{"value": value}, timestamp)
			if err == nil {
				output <- y
			}
		}
		if !dev.excludeMetrics["rocm_temp_mem"] {
			value := metrics.Temperature_mem
			y, err := lp.NewMessage("rocm_temp_mem", dev.tags, dev.meta, map[string]any{"value": value}, timestamp)
			if err == nil {
				output <- y
			}
		}
		if !dev.excludeMetrics["rocm_temp_hotspot"] {
			value := metrics.Temperature_hotspot
			y, err := lp.NewMessage("rocm_temp_hotspot", dev.tags, dev.meta, map[string]any{"value": value}, timestamp)
			if err == nil {
				output <- y
			}
		}
		if !dev.excludeMetrics["rocm_temp_edge"] {
			value := metrics.Temperature_edge
			y, err := lp.NewMessage("rocm_temp_edge", dev.tags, dev.meta, map[string]any{"value": value}, timestamp)
			if err == nil {
				output <- y
			}
		}
		if !dev.excludeMetrics["rocm_temp_vrgfx"] {
			value := metrics.Temperature_vrgfx
			y, err := lp.NewMessage("rocm_temp_vrgfx", dev.tags, dev.meta, map[string]any{"value": value}, timestamp)
			if err == nil {
				output <- y
			}
		}
		if !dev.excludeMetrics["rocm_temp_vrsoc"] {
			value := metrics.Temperature_vrsoc
			y, err := lp.NewMessage("rocm_temp_vrsoc", dev.tags, dev.meta, map[string]any{"value": value}, timestamp)
			if err == nil {
				output <- y
			}
		}
		if !dev.excludeMetrics["rocm_temp_vrmem"] {
			value := metrics.Temperature_vrmem
			y, err := lp.NewMessage("rocm_temp_vrmem", dev.tags, dev.meta, map[string]any{"value": value}, timestamp)
			if err == nil {
				output <- y
			}
		}
		if !dev.excludeMetrics["rocm_gfx_clock"] {
			value := metrics.Average_gfxclk_frequency
			y, err := lp.NewMessage("rocm_gfx_clock", dev.tags, dev.meta, map[string]any{"value": value}, timestamp)
			if err == nil {
				output <- y
			}
		}
		if !dev.excludeMetrics["rocm_soc_clock"] {
			value := metrics.Average_socclk_frequency
			y, err := lp.NewMessage("rocm_soc_clock", dev.tags, dev.meta, map[string]any{"value": value}, timestamp)
			if err == nil {
				output <- y
			}
		}
		if !dev.excludeMetrics["rocm_u_clock"] {
			value := metrics.Average_uclk_frequency
			y, err := lp.NewMessage("rocm_u_clock", dev.tags, dev.meta, map[string]any{"value": value}, timestamp)
			if err == nil {
				output <- y
			}
		}
		if !dev.excludeMetrics["rocm_v0_clock"] {
			value := metrics.Average_vclk0_frequency
			y, err := lp.NewMessage("rocm_v0_clock", dev.tags, dev.meta, map[string]any{"value": value}, timestamp)
			if err == nil {
				output <- y
			}
		}
		if !dev.excludeMetrics["rocm_v1_clock"] {
			value := metrics.Average_vclk1_frequency
			y, err := lp.NewMessage("rocm_v1_clock", dev.tags, dev.meta, map[string]any{"value": value}, timestamp)
			if err == nil {
				output <- y
			}
		}
		if !dev.excludeMetrics["rocm_d0_clock"] {
			value := metrics.Average_dclk0_frequency
			y, err := lp.NewMessage("rocm_d0_clock", dev.tags, dev.meta, map[string]any{"value": value}, timestamp)
			if err == nil {
				output <- y
			}
		}
		if !dev.excludeMetrics["rocm_d1_clock"] {
			value := metrics.Average_dclk1_frequency
			y, err := lp.NewMessage("rocm_d1_clock", dev.tags, dev.meta, map[string]any{"value": value}, timestamp)
			if err == nil {
				output <- y
			}
		}
		if !dev.excludeMetrics["rocm_temp_hbm"] {
			for i := range rocm_smi.NUM_HBM_INSTANCES {
				value := metrics.Temperature_hbm[i]
				y, err := lp.NewMessage("rocm_temp_hbm", dev.tags, dev.meta, map[string]any{"value": value}, timestamp)
				if err == nil {
					y.AddTag("stype", "device")
					y.AddTag("stype-id", fmt.Sprintf("%d", i))
					output <- y
				}
			}
		}
	}

}

// Close metric collector: close network connection, close files, close libraries, ...
// Called once by the collector manager
func (m *RocmSmiCollector) Close() {
	// Unset flag
	ret := rocm_smi.Shutdown()
	if ret != rocm_smi.STATUS_SUCCESS {
		cclog.ComponentError(m.name, "Failed to shutdown ROCm SMI library")
	}
	m.init = false
}
