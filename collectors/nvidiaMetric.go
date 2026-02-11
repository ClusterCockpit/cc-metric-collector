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
	"log"
	"maps"
	"slices"
	"strings"
	"time"

	cclog "github.com/ClusterCockpit/cc-lib/v2/ccLogger"
	lp "github.com/ClusterCockpit/cc-lib/v2/ccMessage"
	"github.com/NVIDIA/go-nvml/pkg/nvml"
)

type NvidiaCollectorConfig struct {
	ExcludeMetrics        []string `json:"exclude_metrics,omitempty"`
	ExcludeDevices        []string `json:"exclude_devices,omitempty"`
	AddPciInfoTag         bool     `json:"add_pci_info_tag,omitempty"`
	UsePciInfoAsTypeId    bool     `json:"use_pci_info_as_type_id,omitempty"`
	AddUuidMeta           bool     `json:"add_uuid_meta,omitempty"`
	AddBoardNumberMeta    bool     `json:"add_board_number_meta,omitempty"`
	AddSerialMeta         bool     `json:"add_serial_meta,omitempty"`
	ProcessMigDevices     bool     `json:"process_mig_devices,omitempty"`
	UseUuidForMigDevices  bool     `json:"use_uuid_for_mig_device,omitempty"`
	UseSliceForMigDevices bool     `json:"use_slice_for_mig_device,omitempty"`
}

type NvidiaCollectorDevice struct {
	device              nvml.Device
	excludeMetrics      map[string]bool
	tags                map[string]string
	meta                map[string]string
	lastEnergyReading   uint64
	lastEnergyTimestamp time.Time
}

type NvidiaCollector struct {
	metricCollector
	config   NvidiaCollectorConfig
	gpus     []NvidiaCollectorDevice
	num_gpus int
}

func (m *NvidiaCollector) CatchPanic() {
	if rerr := recover(); rerr != nil {
		log.Print(rerr)
		m.init = false
	}
}

func (m *NvidiaCollector) Init(config json.RawMessage) error {
	var err error
	m.name = "NvidiaCollector"
	m.config.AddPciInfoTag = false
	m.config.UsePciInfoAsTypeId = false
	m.config.ProcessMigDevices = false
	m.config.UseUuidForMigDevices = false
	m.config.UseSliceForMigDevices = false
	if err := m.setup(); err != nil {
		return fmt.Errorf("%s Init(): setup() call failed: %w", m.name, err)
	}
	if len(config) > 0 {
		err = json.Unmarshal(config, &m.config)
		if err != nil {
			return err
		}
	}
	m.meta = map[string]string{
		"source": m.name,
		"group":  "Nvidia",
	}

	defer m.CatchPanic()

	// Initialize NVIDIA Management Library (NVML)
	ret := nvml.Init()

	// Error: NVML library not found
	// (nvml.ErrorString can not be used in this case)
	if ret == nvml.ERROR_LIBRARY_NOT_FOUND {
		err = fmt.Errorf("NVML library not found")
		cclog.ComponentError(m.name, err.Error())
		return err
	}
	if ret != nvml.SUCCESS {
		err = errors.New(nvml.ErrorString(ret))
		cclog.ComponentError(m.name, "Unable to initialize NVML", err.Error())
		return err
	}

	// Number of NVIDIA GPUs
	num_gpus, ret := nvml.DeviceGetCount()
	if ret != nvml.SUCCESS {
		err = errors.New(nvml.ErrorString(ret))
		cclog.ComponentError(m.name, "Unable to get device count", err.Error())
		return err
	}

	// For all GPUs
	idx := 0
	m.gpus = make([]NvidiaCollectorDevice, num_gpus)
	for i := range num_gpus {

		// Skip excluded devices by ID
		str_i := fmt.Sprintf("%d", i)
		if slices.Contains(m.config.ExcludeDevices, str_i) {
			cclog.ComponentDebug(m.name, "Skipping excluded device", str_i)
			continue
		}

		// Get device handle
		device, ret := nvml.DeviceGetHandleByIndex(i)
		if ret != nvml.SUCCESS {
			err = errors.New(nvml.ErrorString(ret))
			cclog.ComponentError(m.name, "Unable to get device at index", i, ":", err.Error())
			continue
		}

		// Get device's PCI info
		pciInfo, ret := nvml.DeviceGetPciInfo(device)
		if ret != nvml.SUCCESS {
			err = errors.New(nvml.ErrorString(ret))
			cclog.ComponentError(m.name, "Unable to get PCI info for device at index", i, ":", err.Error())
			continue
		}
		// Create PCI ID in the common format used by the NVML.
		pci_id := fmt.Sprintf(
			nvml.DEVICE_PCI_BUS_ID_FMT,
			pciInfo.Domain,
			pciInfo.Bus,
			pciInfo.Device)

		// Skip excluded devices specified by PCI ID
		if slices.Contains(m.config.ExcludeDevices, pci_id) {
			cclog.ComponentDebug(m.name, "Skipping excluded device", pci_id)
			continue
		}

		// Select which value to use as 'type-id'.
		// The PCI ID is commonly required in SLURM environments because the
		// numberic IDs used by SLURM and the ones used by NVML might differ
		// depending on the job type. The PCI ID is more reliable but is commonly
		// not recorded for a job, so it must be added manually in prologue or epilogue
		// e.g. to the comment field
		tid := str_i
		if m.config.UsePciInfoAsTypeId {
			tid = pci_id
		}

		// Now we got all infos together, populate the device list
		g := &m.gpus[idx]

		// Add device handle
		g.device = device
		g.lastEnergyReading = 0
		g.lastEnergyTimestamp = time.Now()

		// Add tags
		g.tags = map[string]string{
			"type":    "accelerator",
			"type-id": tid,
		}

		// Add PCI info as tag if not already used as 'type-id'
		if m.config.AddPciInfoTag && !m.config.UsePciInfoAsTypeId {
			g.tags["pci_identifier"] = pci_id
		}

		g.meta = map[string]string{
			"source": m.name,
			"group":  "Nvidia",
		}

		if m.config.AddBoardNumberMeta {
			board, ret := nvml.DeviceGetBoardPartNumber(device)
			if ret != nvml.SUCCESS {
				cclog.ComponentError(m.name, "Unable to get boart part number for device at index", i, ":", err.Error())
			} else {
				g.meta["board_number"] = board
			}
		}
		if m.config.AddSerialMeta {
			serial, ret := nvml.DeviceGetSerial(device)
			if ret != nvml.SUCCESS {
				cclog.ComponentError(m.name, "Unable to get serial number for device at index", i, ":", err.Error())
			} else {
				g.meta["serial"] = serial
			}
		}
		if m.config.AddUuidMeta {
			uuid, ret := nvml.DeviceGetUUID(device)
			if ret != nvml.SUCCESS {
				cclog.ComponentError(m.name, "Unable to get UUID for device at index", i, ":", err.Error())
			} else {
				g.meta["uuid"] = uuid
			}
		}

		// Add excluded metrics
		g.excludeMetrics = map[string]bool{}
		for _, e := range m.config.ExcludeMetrics {
			g.excludeMetrics[e] = true
		}

		// Increment the index for the next device
		idx++
	}
	m.num_gpus = idx

	m.init = true
	return nil
}

func readMemoryInfo(device *NvidiaCollectorDevice, output chan lp.CCMessage) error {
	if !device.excludeMetrics["nv_fb_mem_total"] || !device.excludeMetrics["nv_fb_mem_used"] || !device.excludeMetrics["nv_fb_mem_reserved"] {
		var total uint64
		var used uint64
		var reserved uint64 = 0
		v2 := false
		meminfo, ret := nvml.DeviceGetMemoryInfo(device.device)
		if ret != nvml.SUCCESS {
			err := errors.New(nvml.ErrorString(ret))
			return err
		}
		// Total physical device memory (in bytes)
		total = meminfo.Total
		// Sum of Reserved and Allocated device memory (in bytes)
		used = meminfo.Used

		if !device.excludeMetrics["nv_fb_mem_total"] {
			t := float64(total) / (1024 * 1024)
			y, err := lp.NewMessage("nv_fb_mem_total", device.tags, device.meta, map[string]any{"value": t}, time.Now())
			if err == nil {
				y.AddMeta("unit", "MByte")
				output <- y
			}
		}

		if !device.excludeMetrics["nv_fb_mem_used"] {
			f := float64(used) / (1024 * 1024)
			y, err := lp.NewMessage("nv_fb_mem_used", device.tags, device.meta, map[string]any{"value": f}, time.Now())
			if err == nil {
				y.AddMeta("unit", "MByte")
				output <- y
			}
		}

		if v2 && !device.excludeMetrics["nv_fb_mem_reserved"] {
			r := float64(reserved) / (1024 * 1024)
			y, err := lp.NewMessage("nv_fb_mem_reserved", device.tags, device.meta, map[string]any{"value": r}, time.Now())
			if err == nil {
				y.AddMeta("unit", "MByte")
				output <- y
			}
		}
	}
	return nil
}

func readBarMemoryInfo(device *NvidiaCollectorDevice, output chan lp.CCMessage) error {
	if !device.excludeMetrics["nv_bar1_mem_total"] || !device.excludeMetrics["nv_bar1_mem_used"] {
		meminfo, ret := nvml.DeviceGetBAR1MemoryInfo(device.device)
		if ret != nvml.SUCCESS {
			err := errors.New(nvml.ErrorString(ret))
			return err
		}
		if !device.excludeMetrics["nv_bar1_mem_total"] {
			t := float64(meminfo.Bar1Total) / (1024 * 1024)
			y, err := lp.NewMessage("nv_bar1_mem_total", device.tags, device.meta, map[string]any{"value": t}, time.Now())
			if err == nil {
				y.AddMeta("unit", "MByte")
				output <- y
			}
		}
		if !device.excludeMetrics["nv_bar1_mem_used"] {
			t := float64(meminfo.Bar1Used) / (1024 * 1024)
			y, err := lp.NewMessage("nv_bar1_mem_used", device.tags, device.meta, map[string]any{"value": t}, time.Now())
			if err == nil {
				y.AddMeta("unit", "MByte")
				output <- y
			}
		}
	}
	return nil
}

func readUtilization(device *NvidiaCollectorDevice, output chan lp.CCMessage) error {
	isMig, ret := nvml.DeviceIsMigDeviceHandle(device.device)
	if ret != nvml.SUCCESS {
		err := errors.New(nvml.ErrorString(ret))
		return err
	}
	if isMig {
		return nil
	}

	if !device.excludeMetrics["nv_util"] || !device.excludeMetrics["nv_mem_util"] {
		// Retrieves the current utilization rates for the device's major subsystems.
		//
		// Available utilization rates
		// * Gpu: Percent of time over the past sample period during which one or more kernels was executing on the GPU.
		// * Memory: Percent of time over the past sample period during which global (device) memory was being read or written
		//
		// Note:
		// * During driver initialization when ECC is enabled one can see high GPU and Memory Utilization readings.
		//   This is caused by ECC Memory Scrubbing mechanism that is performed during driver initialization.
		// * On MIG-enabled GPUs, querying device utilization rates is not currently supported.
		util, ret := nvml.DeviceGetUtilizationRates(device.device)
		if ret == nvml.SUCCESS {
			if !device.excludeMetrics["nv_util"] {
				y, err := lp.NewMessage("nv_util", device.tags, device.meta, map[string]any{"value": float64(util.Gpu)}, time.Now())
				if err == nil {
					y.AddMeta("unit", "%")
					output <- y
				}
			}
			if !device.excludeMetrics["nv_mem_util"] {
				y, err := lp.NewMessage("nv_mem_util", device.tags, device.meta, map[string]any{"value": float64(util.Memory)}, time.Now())
				if err == nil {
					y.AddMeta("unit", "%")
					output <- y
				}
			}
		}
	}
	return nil
}

func readTemp(device *NvidiaCollectorDevice, output chan lp.CCMessage) error {
	if !device.excludeMetrics["nv_temp"] {
		// Retrieves the current temperature readings for the device, in degrees C.
		//
		// Available temperature sensors:
		// * TEMPERATURE_GPU: Temperature sensor for the GPU die.
		// * NVML_TEMPERATURE_COUNT
		temp, ret := nvml.DeviceGetTemperature(device.device, nvml.TEMPERATURE_GPU)
		if ret == nvml.SUCCESS {
			y, err := lp.NewMessage("nv_temp", device.tags, device.meta, map[string]any{"value": float64(temp)}, time.Now())
			if err == nil {
				y.AddMeta("unit", "degC")
				output <- y
			}
		}
	}
	return nil
}

func readFan(device *NvidiaCollectorDevice, output chan lp.CCMessage) error {
	if !device.excludeMetrics["nv_fan"] {
		// Retrieves the intended operating speed of the device's fan.
		//
		// Note: The reported speed is the intended fan speed.
		// If the fan is physically blocked and unable to spin, the output will not match the actual fan speed.
		//
		// For all discrete products with dedicated fans.
		//
		// The fan speed is expressed as a percentage of the product's maximum noise tolerance fan speed.
		// This value may exceed 100% in certain cases.
		fan, ret := nvml.DeviceGetFanSpeed(device.device)
		if ret == nvml.SUCCESS {
			y, err := lp.NewMessage("nv_fan", device.tags, device.meta, map[string]any{"value": float64(fan)}, time.Now())
			if err == nil {
				y.AddMeta("unit", "%")
				output <- y
			}
		}
	}
	return nil
}

// func readFans(device *NvidiaCollectorDevice, output chan lp.CCMessage) error {
// 	if !device.excludeMetrics["nv_fan"] {
// 		numFans, ret := nvml.DeviceGetNumFans(device.device)
// 		if ret == nvml.SUCCESS {
// 			for i := 0; i < numFans; i++ {
// 				fan, ret := nvml.DeviceGetFanSpeed_v2(device.device, i)
// 				if ret == nvml.SUCCESS {
// 					y, err := lp.NewMessage("nv_fan", device.tags, device.meta, map[string]interface{}{"value": float64(fan)}, time.Now())
// 					if err == nil {
// 						y.AddMeta("unit", "%")
// 						y.AddTag("stype", "fan")
// 						y.AddTag("stype-id", fmt.Sprintf("%d", i))
// 						output <- y
// 					}
// 				}
// 			}
// 		}
// 	}
// 	return nil
// }

func readEccMode(device *NvidiaCollectorDevice, output chan lp.CCMessage) error {
	if !device.excludeMetrics["nv_ecc_mode"] {
		// Retrieves the current and pending ECC modes for the device.
		//
		// For Fermi or newer fully supported devices. Only applicable to devices with ECC.
		// Requires NVML_INFOROM_ECC version 1.0 or higher.
		//
		// Changing ECC modes requires a reboot.
		// The "pending" ECC mode refers to the target mode following the next reboot.
		_, ecc_pend, ret := nvml.DeviceGetEccMode(device.device)
		switch ret {
		case nvml.SUCCESS:
			var y lp.CCMessage
			var err error
			switch ecc_pend {
			case nvml.FEATURE_DISABLED:
				y, err = lp.NewMessage("nv_ecc_mode", device.tags, device.meta, map[string]any{"value": "OFF"}, time.Now())
			case nvml.FEATURE_ENABLED:
				y, err = lp.NewMessage("nv_ecc_mode", device.tags, device.meta, map[string]any{"value": "ON"}, time.Now())
			default:
				y, err = lp.NewMessage("nv_ecc_mode", device.tags, device.meta, map[string]any{"value": "UNKNOWN"}, time.Now())
			}
			if err == nil {
				output <- y
			}
		case nvml.ERROR_NOT_SUPPORTED:
			y, err := lp.NewMessage("nv_ecc_mode", device.tags, device.meta, map[string]any{"value": "N/A"}, time.Now())
			if err == nil {
				output <- y
			}
		}
	}
	return nil
}

func readPerfState(device *NvidiaCollectorDevice, output chan lp.CCMessage) error {
	if !device.excludeMetrics["nv_perf_state"] {
		// Retrieves the current performance state for the device.
		//
		// Allowed PStates:
		//  0: Maximum Performance.
		// ..
		// 15: Minimum Performance.
		// 32: Unknown performance state.
		pState, ret := nvml.DeviceGetPerformanceState(device.device)
		if ret == nvml.SUCCESS {
			y, err := lp.NewMessage("nv_perf_state", device.tags, device.meta, map[string]any{"value": fmt.Sprintf("P%d", int(pState))}, time.Now())
			if err == nil {
				output <- y
			}
		}
	}
	return nil
}

func readPowerUsage(device *NvidiaCollectorDevice, output chan lp.CCMessage) error {
	if !device.excludeMetrics["nv_power_usage"] {
		// Retrieves power usage for this GPU in milliwatts and its associated circuitry (e.g. memory)
		//
		// On Fermi and Kepler GPUs the reading is accurate to within +/- 5% of current power draw.
		// On Ampere (except GA100) or newer GPUs, the API returns power averaged over 1 sec interval.
		// On GA100 and older architectures, instantaneous power is returned.
		//
		// It is only available if power management mode is supported.

		mode, ret := nvml.DeviceGetPowerManagementMode(device.device)
		if ret != nvml.SUCCESS {
			return nil
		}
		if mode == nvml.FEATURE_ENABLED {
			power, ret := nvml.DeviceGetPowerUsage(device.device)
			if ret == nvml.SUCCESS {
				y, err := lp.NewMessage("nv_power_usage", device.tags, device.meta, map[string]any{"value": float64(power) / 1000}, time.Now())
				if err == nil {
					y.AddMeta("unit", "watts")
					output <- y
				}
			}
		}
	}
	return nil
}

func readEnergyConsumption(device *NvidiaCollectorDevice, output chan lp.CCMessage) error {
	// Retrieves total energy consumption for this GPU in millijoules (mJ) since the driver was last reloaded

	// For Volta or newer fully supported devices.
	if (!device.excludeMetrics["nv_energy"]) && (!device.excludeMetrics["nv_energy_abs"]) && (!device.excludeMetrics["nv_average_power"]) {
		now := time.Now()
		mode, ret := nvml.DeviceGetPowerManagementMode(device.device)
		if ret != nvml.SUCCESS {
			return nil
		}
		if mode == nvml.FEATURE_ENABLED {
			energy, ret := nvml.DeviceGetTotalEnergyConsumption(device.device)
			if ret == nvml.SUCCESS {
				if device.lastEnergyReading != 0 {
					if !device.excludeMetrics["nv_energy"] {
						y, err := lp.NewMetric("nv_energy", device.tags, device.meta, (energy-device.lastEnergyReading)/1000, now)
						if err == nil {
							y.AddMeta("unit", "Joules")
							output <- y
						}
					}
					if !device.excludeMetrics["nv_average_power"] {

						energyDiff := (energy - device.lastEnergyReading) / 1000
						timeDiff := now.Sub(device.lastEnergyTimestamp)
						y, err := lp.NewMetric("nv_average_power", device.tags, device.meta, energyDiff/uint64(timeDiff.Seconds()), now)
						if err == nil {
							y.AddMeta("unit", "watts")
							output <- y
						}
					}
				}
				if !device.excludeMetrics["nv_energy_abs"] {
					y, err := lp.NewMetric("nv_energy_abs", device.tags, device.meta, energy/1000, now)
					if err == nil {
						y.AddMeta("unit", "Joules")
						output <- y
					}
				}
				device.lastEnergyReading = energy
				device.lastEnergyTimestamp = time.Now()
			}
		}
	}
	return nil
}

func readClocks(device *NvidiaCollectorDevice, output chan lp.CCMessage) error {
	// Retrieves the current clock speeds for the device.
	//
	// Available clock information:
	// * CLOCK_GRAPHICS: Graphics clock domain.
	// * CLOCK_SM: Streaming Multiprocessor clock domain.
	// * CLOCK_MEM: Memory clock domain.
	if !device.excludeMetrics["nv_graphics_clock"] {
		graphicsClock, ret := nvml.DeviceGetClockInfo(device.device, nvml.CLOCK_GRAPHICS)
		if ret == nvml.SUCCESS {
			y, err := lp.NewMessage("nv_graphics_clock", device.tags, device.meta, map[string]any{"value": float64(graphicsClock)}, time.Now())
			if err == nil {
				y.AddMeta("unit", "MHz")
				output <- y
			}
		}
	}

	if !device.excludeMetrics["nv_sm_clock"] {
		smCock, ret := nvml.DeviceGetClockInfo(device.device, nvml.CLOCK_SM)
		if ret == nvml.SUCCESS {
			y, err := lp.NewMessage("nv_sm_clock", device.tags, device.meta, map[string]any{"value": float64(smCock)}, time.Now())
			if err == nil {
				y.AddMeta("unit", "MHz")
				output <- y
			}
		}
	}

	if !device.excludeMetrics["nv_mem_clock"] {
		memClock, ret := nvml.DeviceGetClockInfo(device.device, nvml.CLOCK_MEM)
		if ret == nvml.SUCCESS {
			y, err := lp.NewMessage("nv_mem_clock", device.tags, device.meta, map[string]any{"value": float64(memClock)}, time.Now())
			if err == nil {
				y.AddMeta("unit", "MHz")
				output <- y
			}
		}
	}
	if !device.excludeMetrics["nv_video_clock"] {
		memClock, ret := nvml.DeviceGetClockInfo(device.device, nvml.CLOCK_VIDEO)
		if ret == nvml.SUCCESS {
			y, err := lp.NewMessage("nv_video_clock", device.tags, device.meta, map[string]any{"value": float64(memClock)}, time.Now())
			if err == nil {
				y.AddMeta("unit", "MHz")
				output <- y
			}
		}
	}
	return nil
}

func readMaxClocks(device *NvidiaCollectorDevice, output chan lp.CCMessage) error {
	// Retrieves the maximum clock speeds for the device.
	//
	// Available clock information:
	// * CLOCK_GRAPHICS: Graphics clock domain.
	// * CLOCK_SM:       Streaming multiprocessor clock domain.
	// * CLOCK_MEM:      Memory clock domain.
	// * CLOCK_VIDEO:    Video encoder/decoder clock domain.
	// * CLOCK_COUNT:    Count of clock types.
	//
	// Note:
	/// On GPUs from Fermi family current P0 clocks (reported by nvmlDeviceGetClockInfo) can differ from max clocks by few MHz.
	if !device.excludeMetrics["nv_max_graphics_clock"] {
		max_gclk, ret := nvml.DeviceGetMaxClockInfo(device.device, nvml.CLOCK_GRAPHICS)
		if ret == nvml.SUCCESS {
			y, err := lp.NewMetric("nv_max_graphics_clock", device.tags, device.meta, float64(max_gclk), time.Now())
			if err == nil {
				y.AddMeta("unit", "MHz")
				output <- y
			}
		}
	}

	if !device.excludeMetrics["nv_max_sm_clock"] {
		maxSmClock, ret := nvml.DeviceGetMaxClockInfo(device.device, nvml.CLOCK_SM)
		if ret == nvml.SUCCESS {
			y, err := lp.NewMetric("nv_max_sm_clock", device.tags, device.meta, float64(maxSmClock), time.Now())
			if err == nil {
				y.AddMeta("unit", "MHz")
				output <- y
			}
		}
	}

	if !device.excludeMetrics["nv_max_mem_clock"] {
		maxMemClock, ret := nvml.DeviceGetMaxClockInfo(device.device, nvml.CLOCK_MEM)
		if ret == nvml.SUCCESS {
			y, err := lp.NewMetric("nv_max_mem_clock", device.tags, device.meta, float64(maxMemClock), time.Now())
			if err == nil {
				y.AddMeta("unit", "MHz")
				output <- y
			}
		}
	}

	if !device.excludeMetrics["nv_max_video_clock"] {
		maxVideoClock, ret := nvml.DeviceGetMaxClockInfo(device.device, nvml.CLOCK_VIDEO)
		if ret == nvml.SUCCESS {
			y, err := lp.NewMetric("nv_max_video_clock", device.tags, device.meta, float64(maxVideoClock), time.Now())
			if err == nil {
				y.AddMeta("unit", "MHz")
				output <- y
			}
		}
	}
	return nil
}

func readEccErrors(device *NvidiaCollectorDevice, output chan lp.CCMessage) error {
	if !device.excludeMetrics["nv_ecc_uncorrected_error"] {
		// Retrieves the total ECC error counts for the device.
		//
		// For Fermi or newer fully supported devices.
		// Only applicable to devices with ECC.
		// Requires NVML_INFOROM_ECC version 1.0 or higher.
		// Requires ECC Mode to be enabled.
		//
		// The total error count is the sum of errors across each of the separate memory systems,
		// i.e. the total set of errors across the entire device.
		ecc_db, ret := nvml.DeviceGetTotalEccErrors(device.device, nvml.MEMORY_ERROR_TYPE_UNCORRECTED, nvml.AGGREGATE_ECC)
		if ret == nvml.SUCCESS {
			y, err := lp.NewMessage("nv_ecc_uncorrected_error", device.tags, device.meta, map[string]any{"value": float64(ecc_db)}, time.Now())
			if err == nil {
				output <- y
			}
		}
	}
	if !device.excludeMetrics["nv_ecc_corrected_error"] {
		ecc_sb, ret := nvml.DeviceGetTotalEccErrors(device.device, nvml.MEMORY_ERROR_TYPE_CORRECTED, nvml.AGGREGATE_ECC)
		if ret == nvml.SUCCESS {
			y, err := lp.NewMessage("nv_ecc_corrected_error", device.tags, device.meta, map[string]any{"value": float64(ecc_sb)}, time.Now())
			if err == nil {
				output <- y
			}
		}
	}
	return nil
}

func readPowerLimit(device *NvidiaCollectorDevice, output chan lp.CCMessage) error {
	if !device.excludeMetrics["nv_power_max_limit"] {
		// Retrieves the power management limit associated with this device.
		//
		// For Fermi or newer fully supported devices.
		//
		// The power limit defines the upper boundary for the card's power draw.
		// If the card's total power draw reaches this limit the power management algorithm kicks in.
		pwr_limit, ret := nvml.DeviceGetPowerManagementLimit(device.device)
		if ret == nvml.SUCCESS {
			y, err := lp.NewMessage("nv_power_max_limit", device.tags, device.meta, map[string]any{"value": float64(pwr_limit) / 1000}, time.Now())
			if err == nil {
				y.AddMeta("unit", "watts")
				output <- y
			}
		}
	}
	return nil
}

func readEncUtilization(device *NvidiaCollectorDevice, output chan lp.CCMessage) error {
	isMig, ret := nvml.DeviceIsMigDeviceHandle(device.device)
	if ret != nvml.SUCCESS {
		err := errors.New(nvml.ErrorString(ret))
		return err
	}
	if isMig {
		return nil
	}
	if !device.excludeMetrics["nv_encoder_util"] {
		// Retrieves the current utilization and sampling size in microseconds for the Encoder
		//
		// For Kepler or newer fully supported devices.
		//
		// Note: On MIG-enabled GPUs, querying encoder utilization is not currently supported.
		enc_util, _, ret := nvml.DeviceGetEncoderUtilization(device.device)
		if ret == nvml.SUCCESS {
			y, err := lp.NewMessage("nv_encoder_util", device.tags, device.meta, map[string]any{"value": float64(enc_util)}, time.Now())
			if err == nil {
				y.AddMeta("unit", "%")
				output <- y
			}
		}
	}
	return nil
}

func readDecUtilization(device *NvidiaCollectorDevice, output chan lp.CCMessage) error {
	isMig, ret := nvml.DeviceIsMigDeviceHandle(device.device)
	if ret != nvml.SUCCESS {
		err := errors.New(nvml.ErrorString(ret))
		return err
	}
	if isMig {
		return nil
	}
	if !device.excludeMetrics["nv_decoder_util"] {
		// Retrieves the current utilization and sampling size in microseconds for the Encoder
		//
		// For Kepler or newer fully supported devices.
		//
		// Note: On MIG-enabled GPUs, querying encoder utilization is not currently supported.
		dec_util, _, ret := nvml.DeviceGetDecoderUtilization(device.device)
		if ret == nvml.SUCCESS {
			y, err := lp.NewMessage("nv_decoder_util", device.tags, device.meta, map[string]any{"value": float64(dec_util)}, time.Now())
			if err == nil {
				y.AddMeta("unit", "%")
				output <- y
			}
		}
	}
	return nil
}

func readRemappedRows(device *NvidiaCollectorDevice, output chan lp.CCMessage) error {
	if !device.excludeMetrics["nv_remapped_rows_corrected"] ||
		!device.excludeMetrics["nv_remapped_rows_uncorrected"] ||
		!device.excludeMetrics["nv_remapped_rows_pending"] ||
		!device.excludeMetrics["nv_remapped_rows_failure"] {
		// Get number of remapped rows. The number of rows reported will be based on the cause of the remapping.
		// isPending indicates whether or not there are pending remappings.
		// A reset will be required to actually remap the row.
		// failureOccurred will be set if a row remapping ever failed in the past.
		// A pending remapping won't affect future work on the GPU since error-containment and dynamic page blacklisting will take care of that.
		//
		// For Ampere or newer fully supported devices.
		//
		// Note: On MIG-enabled GPUs with active instances, querying the number of remapped rows is not supported
		corrected, uncorrected, pending, failure, ret := nvml.DeviceGetRemappedRows(device.device)
		if ret == nvml.SUCCESS {
			if !device.excludeMetrics["nv_remapped_rows_corrected"] {
				y, err := lp.NewMessage("nv_remapped_rows_corrected", device.tags, device.meta, map[string]any{"value": float64(corrected)}, time.Now())
				if err == nil {
					output <- y
				}
			}
			if !device.excludeMetrics["nv_remapped_rows_uncorrected"] {
				y, err := lp.NewMessage("nv_remapped_rows_corrected", device.tags, device.meta, map[string]any{"value": float64(uncorrected)}, time.Now())
				if err == nil {
					output <- y
				}
			}
			if !device.excludeMetrics["nv_remapped_rows_pending"] {
				p := 0
				if pending {
					p = 1
				}
				y, err := lp.NewMessage("nv_remapped_rows_pending", device.tags, device.meta, map[string]any{"value": p}, time.Now())
				if err == nil {
					output <- y
				}
			}
			if !device.excludeMetrics["nv_remapped_rows_failure"] {
				f := 0
				if failure {
					f = 1
				}
				y, err := lp.NewMessage("nv_remapped_rows_failure", device.tags, device.meta, map[string]any{"value": f}, time.Now())
				if err == nil {
					output <- y
				}
			}
		}
	}
	return nil
}

func readProcessCounts(device *NvidiaCollectorDevice, output chan lp.CCMessage) error {
	if !device.excludeMetrics["nv_compute_processes"] {
		// Get information about processes with a compute context on a device
		//
		// For Fermi &tm; or newer fully supported devices.
		//
		// This function returns information only about compute running processes (e.g. CUDA application which have
		// active context). Any graphics applications (e.g. using OpenGL, DirectX) won't be listed by this function.
		//
		// To query the current number of running compute processes, call this function with *infoCount = 0. The
		// return code will be NVML_ERROR_INSUFFICIENT_SIZE, or NVML_SUCCESS if none are running. For this call
		// \a infos is allowed to be NULL.
		//
		// The usedGpuMemory field returned is all of the memory used by the application.
		//
		// Keep in mind that information returned by this call is dynamic and the number of elements might change in
		// time. Allocate more space for \a infos table in case new compute processes are spawned.
		//
		// @note In MIG mode, if device handle is provided, the API returns aggregate information, only if
		//        the caller has appropriate privileges. Per-instance information can be queried by using
		//        specific MIG device handles.
		//        Querying per-instance information using MIG device handles is not supported if the device is in vGPU Host virtualization mode.
		procList, ret := nvml.DeviceGetComputeRunningProcesses(device.device)
		if ret == nvml.SUCCESS {
			y, err := lp.NewMessage("nv_compute_processes", device.tags, device.meta, map[string]any{"value": len(procList)}, time.Now())
			if err == nil {
				output <- y
			}
		}
	}
	if !device.excludeMetrics["nv_graphics_processes"] {
		// Get information about processes with a graphics context on a device
		//
		// For Kepler &tm; or newer fully supported devices.
		//
		// This function returns information only about graphics based processes
		// (eg. applications using OpenGL, DirectX)
		//
		// To query the current number of running graphics processes, call this function with *infoCount = 0. The
		// return code will be NVML_ERROR_INSUFFICIENT_SIZE, or NVML_SUCCESS if none are running. For this call
		// \a infos is allowed to be NULL.
		//
		// The usedGpuMemory field returned is all of the memory used by the application.
		//
		// Keep in mind that information returned by this call is dynamic and the number of elements might change in
		// time. Allocate more space for \a infos table in case new graphics processes are spawned.
		//
		// @note In MIG mode, if device handle is provided, the API returns aggregate information, only if
		//       the caller has appropriate privileges. Per-instance information can be queried by using
		//       specific MIG device handles.
		//       Querying per-instance information using MIG device handles is not supported if the device is in vGPU Host virtualization mode.
		procList, ret := nvml.DeviceGetGraphicsRunningProcesses(device.device)
		if ret == nvml.SUCCESS {
			y, err := lp.NewMessage("nv_graphics_processes", device.tags, device.meta, map[string]any{"value": len(procList)}, time.Now())
			if err == nil {
				output <- y
			}
		}
	}
	// if !device.excludeMetrics["nv_mps_compute_processes"] {
	// 	// Get information about processes with a MPS compute context on a device
	// 	//
	// 	// For Volta &tm; or newer fully supported devices.
	// 	//
	// 	// This function returns information only about compute running processes (e.g. CUDA application which have
	// 	// active context) utilizing MPS. Any graphics applications (e.g. using OpenGL, DirectX) won't be listed by
	// 	// this function.
	// 	//
	// 	// To query the current number of running compute processes, call this function with *infoCount = 0. The
	// 	// return code will be NVML_ERROR_INSUFFICIENT_SIZE, or NVML_SUCCESS if none are running. For this call
	// 	// \a infos is allowed to be NULL.
	// 	//
	// 	// The usedGpuMemory field returned is all of the memory used by the application.
	// 	//
	// 	// Keep in mind that information returned by this call is dynamic and the number of elements might change in
	// 	// time. Allocate more space for \a infos table in case new compute processes are spawned.
	// 	//
	// 	// @note In MIG mode, if device handle is provided, the API returns aggregate information, only if
	// 	//       the caller has appropriate privileges. Per-instance information can be queried by using
	// 	//       specific MIG device handles.
	// 	//       Querying per-instance information using MIG device handles is not supported if the device is in vGPU Host virtualization mode.
	// 	procList, ret := nvml.DeviceGetMPSComputeRunningProcesses(device.device)
	// 	if ret == nvml.SUCCESS {
	// 		y, err := lp.NewMessage("nv_mps_compute_processes", device.tags, device.meta, map[string]interface{}{"value": len(procList)}, time.Now())
	// 		if err == nil {
	// 			output <- y
	// 		}
	// 	}
	// }
	return nil
}

func readViolationStats(device *NvidiaCollectorDevice, output chan lp.CCMessage) error {
	var violTime nvml.ViolationTime
	var ret nvml.Return

	// Gets the duration of time during which the device was throttled (lower than requested clocks) due to power
	//  or thermal constraints.
	//
	// The method is important to users who are tying to understand if their GPUs throttle at any point during their applications. The
	// difference in violation times at two different reference times gives the indication of GPU throttling event.
	//
	// Violation for thermal capping is not supported at this time.
	//
	// For Kepler  or newer fully supported devices.

	if !device.excludeMetrics["nv_violation_power"] {
		// How long did power violations cause the GPU to be below application clocks
		violTime, ret = nvml.DeviceGetViolationStatus(device.device, nvml.PERF_POLICY_POWER)
		if ret == nvml.SUCCESS {
			t := float64(violTime.ViolationTime) * 1e-9
			y, err := lp.NewMessage("nv_violation_power", device.tags, device.meta, map[string]any{"value": t}, time.Now())
			if err == nil {
				y.AddMeta("unit", "sec")
				output <- y
			}
		}
	}
	if !device.excludeMetrics["nv_violation_thermal"] {
		// How long did thermal violations cause the GPU to be below application clocks
		violTime, ret = nvml.DeviceGetViolationStatus(device.device, nvml.PERF_POLICY_THERMAL)
		if ret == nvml.SUCCESS {
			t := float64(violTime.ViolationTime) * 1e-9
			y, err := lp.NewMessage("nv_violation_thermal", device.tags, device.meta, map[string]any{"value": t}, time.Now())
			if err == nil {
				y.AddMeta("unit", "sec")
				output <- y
			}
		}
	}
	if !device.excludeMetrics["nv_violation_sync_boost"] {
		// How long did sync boost cause the GPU to be below application clocks
		violTime, ret = nvml.DeviceGetViolationStatus(device.device, nvml.PERF_POLICY_SYNC_BOOST)
		if ret == nvml.SUCCESS {
			t := float64(violTime.ViolationTime) * 1e-9
			y, err := lp.NewMessage("nv_violation_sync_boost", device.tags, device.meta, map[string]any{"value": t}, time.Now())
			if err == nil {
				y.AddMeta("unit", "sec")
				output <- y
			}
		}
	}
	if !device.excludeMetrics["nv_violation_board_limit"] {
		// How long did the board limit cause the GPU to be below application clocks
		violTime, ret = nvml.DeviceGetViolationStatus(device.device, nvml.PERF_POLICY_BOARD_LIMIT)
		if ret == nvml.SUCCESS {
			t := float64(violTime.ViolationTime) * 1e-9
			y, err := lp.NewMessage("nv_violation_board_limit", device.tags, device.meta, map[string]any{"value": t}, time.Now())
			if err == nil {
				y.AddMeta("unit", "sec")
				output <- y
			}
		}
	}
	if !device.excludeMetrics["nv_violation_low_util"] {
		// How long did low utilization cause the GPU to be below application clocks
		violTime, ret = nvml.DeviceGetViolationStatus(device.device, nvml.PERF_POLICY_LOW_UTILIZATION)
		if ret == nvml.SUCCESS {
			t := float64(violTime.ViolationTime) * 1e-9
			y, err := lp.NewMessage("nv_violation_low_util", device.tags, device.meta, map[string]any{"value": t}, time.Now())
			if err == nil {
				y.AddMeta("unit", "sec")
				output <- y
			}
		}
	}
	if !device.excludeMetrics["nv_violation_reliability"] {
		// How long did the board reliability limit cause the GPU to be below application clocks
		violTime, ret = nvml.DeviceGetViolationStatus(device.device, nvml.PERF_POLICY_RELIABILITY)
		if ret == nvml.SUCCESS {
			t := float64(violTime.ViolationTime) * 1e-9
			y, err := lp.NewMessage("nv_violation_reliability", device.tags, device.meta, map[string]any{"value": t}, time.Now())
			if err == nil {
				y.AddMeta("unit", "sec")
				output <- y
			}
		}
	}
	if !device.excludeMetrics["nv_violation_below_app_clock"] {
		// Total time the GPU was held below application clocks by any limiter (all of above)
		violTime, ret = nvml.DeviceGetViolationStatus(device.device, nvml.PERF_POLICY_TOTAL_APP_CLOCKS)
		if ret == nvml.SUCCESS {
			t := float64(violTime.ViolationTime) * 1e-9
			y, err := lp.NewMessage("nv_violation_below_app_clock", device.tags, device.meta, map[string]any{"value": t}, time.Now())
			if err == nil {
				y.AddMeta("unit", "sec")
				output <- y
			}
		}
	}
	if !device.excludeMetrics["nv_violation_below_base_clock"] {
		// Total time the GPU was held below base clocks
		violTime, ret = nvml.DeviceGetViolationStatus(device.device, nvml.PERF_POLICY_TOTAL_BASE_CLOCKS)
		if ret == nvml.SUCCESS {
			t := float64(violTime.ViolationTime) * 1e-9
			y, err := lp.NewMessage("nv_violation_below_base_clock", device.tags, device.meta, map[string]any{"value": t}, time.Now())
			if err == nil {
				y.AddMeta("unit", "sec")
				output <- y
			}
		}
	}

	return nil
}

func readNVLinkStats(device *NvidiaCollectorDevice, output chan lp.CCMessage) error {
	// Retrieves the specified error counter value
	// Please refer to \a nvmlNvLinkErrorCounter_t for error counters that are available
	//
	// For Pascal &tm; or newer fully supported devices.

	var aggregate_crc_errors uint64 = 0
	var aggregate_ecc_errors uint64 = 0
	var aggregate_replay_errors uint64 = 0
	var aggregate_recovery_errors uint64 = 0
	var aggregate_crc_flit_errors uint64 = 0

	for i := range nvml.NVLINK_MAX_LINKS {
		state, ret := nvml.DeviceGetNvLinkState(device.device, i)
		if ret == nvml.SUCCESS {
			if state == nvml.FEATURE_ENABLED {
				if !device.excludeMetrics["nv_nvlink_crc_errors"] {
					// Data link receive data CRC error counter
					count, ret := nvml.DeviceGetNvLinkErrorCounter(device.device, i, nvml.NVLINK_ERROR_DL_CRC_DATA)
					aggregate_crc_errors += count
					if ret == nvml.SUCCESS {
						y, err := lp.NewMessage("nv_nvlink_crc_errors", device.tags, device.meta, map[string]any{"value": count}, time.Now())
						if err == nil {
							y.AddTag("stype", "nvlink")
							y.AddTag("stype-id", fmt.Sprintf("%d", i))
							output <- y
						}
					}
				}
				if !device.excludeMetrics["nv_nvlink_ecc_errors"] {
					// Data link receive data ECC error counter
					count, ret := nvml.DeviceGetNvLinkErrorCounter(device.device, i, nvml.NVLINK_ERROR_DL_ECC_DATA)
					aggregate_ecc_errors += count
					if ret == nvml.SUCCESS {
						y, err := lp.NewMessage("nv_nvlink_ecc_errors", device.tags, device.meta, map[string]any{"value": count}, time.Now())
						if err == nil {
							y.AddTag("stype", "nvlink")
							y.AddTag("stype-id", fmt.Sprintf("%d", i))
							output <- y
						}
					}
				}
				if !device.excludeMetrics["nv_nvlink_replay_errors"] {
					// Data link transmit replay error counter
					count, ret := nvml.DeviceGetNvLinkErrorCounter(device.device, i, nvml.NVLINK_ERROR_DL_REPLAY)
					aggregate_replay_errors += count
					if ret == nvml.SUCCESS {
						y, err := lp.NewMessage("nv_nvlink_replay_errors", device.tags, device.meta, map[string]any{"value": count}, time.Now())
						if err == nil {
							y.AddTag("stype", "nvlink")
							y.AddTag("stype-id", fmt.Sprintf("%d", i))
							output <- y
						}
					}
				}
				if !device.excludeMetrics["nv_nvlink_recovery_errors"] {
					// Data link transmit recovery error counter
					count, ret := nvml.DeviceGetNvLinkErrorCounter(device.device, i, nvml.NVLINK_ERROR_DL_RECOVERY)
					aggregate_recovery_errors += count
					if ret == nvml.SUCCESS {
						y, err := lp.NewMessage("nv_nvlink_recovery_errors", device.tags, device.meta, map[string]any{"value": count}, time.Now())
						if err == nil {
							y.AddTag("stype", "nvlink")
							y.AddTag("stype-id", fmt.Sprintf("%d", i))
							output <- y
						}
					}
				}
				if !device.excludeMetrics["nv_nvlink_crc_flit_errors"] {
					// Data link receive flow control digit CRC error counter
					count, ret := nvml.DeviceGetNvLinkErrorCounter(device.device, i, nvml.NVLINK_ERROR_DL_CRC_FLIT)
					aggregate_crc_flit_errors += count
					if ret == nvml.SUCCESS {
						y, err := lp.NewMessage("nv_nvlink_crc_flit_errors", device.tags, device.meta, map[string]any{"value": count}, time.Now())
						if err == nil {
							y.AddTag("stype", "nvlink")
							y.AddTag("stype-id", fmt.Sprintf("%d", i))
							output <- y
						}
					}
				}
			}
		}
	}

	// Export aggegated values
	if !device.excludeMetrics["nv_nvlink_crc_errors"] {
		// Data link receive data CRC error counter
		y, err := lp.NewMessage("nv_nvlink_crc_errors_sum", device.tags, device.meta, map[string]any{"value": aggregate_crc_errors}, time.Now())
		if err == nil {
			y.AddTag("stype", "nvlink")
			output <- y
		}
	}
	if !device.excludeMetrics["nv_nvlink_ecc_errors"] {
		// Data link receive data ECC error counter
		y, err := lp.NewMessage("nv_nvlink_ecc_errors_sum", device.tags, device.meta, map[string]any{"value": aggregate_ecc_errors}, time.Now())
		if err == nil {
			y.AddTag("stype", "nvlink")
			output <- y
		}
	}
	if !device.excludeMetrics["nv_nvlink_replay_errors"] {
		// Data link transmit replay error counter
		y, err := lp.NewMessage("nv_nvlink_replay_errors_sum", device.tags, device.meta, map[string]any{"value": aggregate_replay_errors}, time.Now())
		if err == nil {
			y.AddTag("stype", "nvlink")
			output <- y
		}
	}
	if !device.excludeMetrics["nv_nvlink_recovery_errors"] {
		// Data link transmit recovery error counter
		y, err := lp.NewMessage("nv_nvlink_recovery_errors_sum", device.tags, device.meta, map[string]any{"value": aggregate_recovery_errors}, time.Now())
		if err == nil {
			y.AddTag("stype", "nvlink")
			output <- y
		}
	}
	if !device.excludeMetrics["nv_nvlink_crc_flit_errors"] {
		// Data link receive flow control digit CRC error counter
		y, err := lp.NewMessage("nv_nvlink_crc_flit_errors_sum", device.tags, device.meta, map[string]any{"value": aggregate_crc_flit_errors}, time.Now())
		if err == nil {
			y.AddTag("stype", "nvlink")
			output <- y
		}
	}
	return nil
}

func (m *NvidiaCollector) Read(interval time.Duration, output chan lp.CCMessage) {
	var err error
	if !m.init {
		return
	}

	readAll := func(device *NvidiaCollectorDevice, output chan lp.CCMessage) {
		name, ret := nvml.DeviceGetName(device.device)
		if ret != nvml.SUCCESS {
			name = "NoName"
		}
		err = readMemoryInfo(device, output)
		if err != nil {
			cclog.ComponentDebug(m.name, "readMemoryInfo for device", name, "failed")
		}

		err = readUtilization(device, output)
		if err != nil {
			cclog.ComponentDebug(m.name, "readUtilization for device", name, "failed")
		}

		err = readTemp(device, output)
		if err != nil {
			cclog.ComponentDebug(m.name, "readTemp for device", name, "failed")
		}

		err = readFan(device, output)
		if err != nil {
			cclog.ComponentDebug(m.name, "readFan for device", name, "failed")
		}

		err = readEccMode(device, output)
		if err != nil {
			cclog.ComponentDebug(m.name, "readEccMode for device", name, "failed")
		}

		err = readPerfState(device, output)
		if err != nil {
			cclog.ComponentDebug(m.name, "readPerfState for device", name, "failed")
		}

		err = readPowerUsage(device, output)
		if err != nil {
			cclog.ComponentDebug(m.name, "readPowerUsage for device", name, "failed")
		}

		err = readEnergyConsumption(device, output)
		if err != nil {
			cclog.ComponentDebug(m.name, "readEnergyConsumption for device", name, "failed")
		}

		err = readClocks(device, output)
		if err != nil {
			cclog.ComponentDebug(m.name, "readClocks for device", name, "failed")
		}

		err = readMaxClocks(device, output)
		if err != nil {
			cclog.ComponentDebug(m.name, "readMaxClocks for device", name, "failed")
		}

		err = readEccErrors(device, output)
		if err != nil {
			cclog.ComponentDebug(m.name, "readEccErrors for device", name, "failed")
		}

		err = readPowerLimit(device, output)
		if err != nil {
			cclog.ComponentDebug(m.name, "readPowerLimit for device", name, "failed")
		}

		err = readEncUtilization(device, output)
		if err != nil {
			cclog.ComponentDebug(m.name, "readEncUtilization for device", name, "failed")
		}

		err = readDecUtilization(device, output)
		if err != nil {
			cclog.ComponentDebug(m.name, "readDecUtilization for device", name, "failed")
		}

		err = readRemappedRows(device, output)
		if err != nil {
			cclog.ComponentDebug(m.name, "readRemappedRows for device", name, "failed")
		}

		err = readBarMemoryInfo(device, output)
		if err != nil {
			cclog.ComponentDebug(m.name, "readBarMemoryInfo for device", name, "failed")
		}

		err = readProcessCounts(device, output)
		if err != nil {
			cclog.ComponentDebug(m.name, "readProcessCounts for device", name, "failed")
		}

		err = readViolationStats(device, output)
		if err != nil {
			cclog.ComponentDebug(m.name, "readViolationStats for device", name, "failed")
		}

		err = readNVLinkStats(device, output)
		if err != nil {
			cclog.ComponentDebug(m.name, "readNVLinkStats for device", name, "failed")
		}
	}

	// Actual read loop over all attached Nvidia GPUs
	for i := 0; i < m.num_gpus; i++ {

		readAll(&m.gpus[i], output)

		// Iterate over all MIG devices if any
		if m.config.ProcessMigDevices {
			current, _, ret := nvml.DeviceGetMigMode(m.gpus[i].device)
			if ret != nvml.SUCCESS {
				continue
			}
			if current == nvml.DEVICE_MIG_DISABLE {
				continue
			}

			maxMig, ret := nvml.DeviceGetMaxMigDeviceCount(m.gpus[i].device)
			if ret != nvml.SUCCESS {
				continue
			}
			if maxMig == 0 {
				continue
			}
			cclog.ComponentDebug(m.name, "Reading MIG devices for GPU", i)

			for j := range maxMig {
				mdev, ret := nvml.DeviceGetMigDeviceHandleByIndex(m.gpus[i].device, j)
				if ret != nvml.SUCCESS {
					continue
				}

				excludeMetrics := make(map[string]bool)
				for _, metric := range m.config.ExcludeMetrics {
					excludeMetrics[metric] = true
				}

				migDevice := NvidiaCollectorDevice{
					device:         mdev,
					tags:           map[string]string{},
					meta:           map[string]string{},
					excludeMetrics: excludeMetrics,
				}
				maps.Copy(migDevice.tags, m.gpus[i].tags)
				migDevice.tags["stype"] = "mig"
				if m.config.UseUuidForMigDevices {
					uuid, ret := nvml.DeviceGetUUID(mdev)
					if ret != nvml.SUCCESS {
						cclog.ComponentError(m.name, "Unable to get UUID for mig device at index", j, ":", err.Error())
					} else {
						migDevice.tags["stype-id"] = uuid
					}
				} else if m.config.UseSliceForMigDevices {
					name, ret := nvml.DeviceGetName(m.gpus[i].device)
					if ret == nvml.SUCCESS {
						mname, ret := nvml.DeviceGetName(mdev)
						if ret == nvml.SUCCESS {
							x := strings.ReplaceAll(mname, name, "")
							x = strings.ReplaceAll(x, "MIG", "")
							x = strings.TrimSpace(x)
							migDevice.tags["stype-id"] = x
						}
					}
				}
				if _, ok := migDevice.tags["stype-id"]; !ok {
					migDevice.tags["stype-id"] = fmt.Sprintf("%d", j)
				}
				maps.Copy(migDevice.meta, m.gpus[i].meta)
				if _, ok := migDevice.meta["uuid"]; ok && !m.config.UseUuidForMigDevices {
					uuid, ret := nvml.DeviceGetUUID(mdev)
					if ret == nvml.SUCCESS {
						migDevice.meta["uuid"] = uuid
					}
				}

				readAll(&migDevice, output)
			}
		}
	}
}

func (m *NvidiaCollector) Close() {
	if m.init {
		if ret := nvml.Shutdown(); ret != nvml.SUCCESS {
			cclog.ComponentError(m.name, "nvml.Shutdown() not successful")
		}
		m.init = false
	}
}
