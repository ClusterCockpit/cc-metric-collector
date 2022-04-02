package collectors

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	cclog "github.com/ClusterCockpit/cc-metric-collector/internal/ccLogger"
	lp "github.com/ClusterCockpit/cc-metric-collector/internal/ccMetric"
	stats "github.com/ClusterCockpit/cc-metric-collector/internal/metricRouter"
	"github.com/NVIDIA/go-nvml/pkg/nvml"
)

type NvidiaCollectorConfig struct {
	ExcludeMetrics []string `json:"exclude_metrics,omitempty"`
	ExcludeDevices []string `json:"exclude_devices,omitempty"`
	AddPciInfoTag  bool     `json:"add_pci_info_tag,omitempty"`
}

type NvidiaCollectorDevice struct {
	device         nvml.Device
	excludeMetrics map[string]bool
	tags           map[string]string
}

type NvidiaCollector struct {
	metricCollector
	num_gpus              int
	config                NvidiaCollectorConfig
	gpus                  []NvidiaCollectorDevice
	statsProcessedMetrics int64
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
	m.setup()
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

	m.num_gpus = 0
	defer m.CatchPanic()

	// Initialize NVIDIA Management Library (NVML)
	ret := nvml.Init()
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
	m.gpus = make([]NvidiaCollectorDevice, num_gpus)
	for i := 0; i < num_gpus; i++ {
		g := &m.gpus[i]

		// Skip excluded devices
		str_i := fmt.Sprintf("%d", i)
		if _, skip := stringArrayContains(m.config.ExcludeDevices, str_i); skip {
			continue
		}

		// Get device handle
		device, ret := nvml.DeviceGetHandleByIndex(i)
		if ret != nvml.SUCCESS {
			err = errors.New(nvml.ErrorString(ret))
			cclog.ComponentError(m.name, "Unable to get device at index", i, ":", err.Error())
			return err
		}
		g.device = device

		// Add tags
		g.tags = map[string]string{
			"type":    "accelerator",
			"type-id": str_i,
		}

		// Add excluded metrics
		g.excludeMetrics = map[string]bool{}
		for _, e := range m.config.ExcludeMetrics {
			g.excludeMetrics[e] = true
		}

		// Add PCI info as tag
		if m.config.AddPciInfoTag {
			pciInfo, ret := nvml.DeviceGetPciInfo(g.device)
			if ret != nvml.SUCCESS {
				err = errors.New(nvml.ErrorString(ret))
				cclog.ComponentError(m.name, "Unable to get PCI info for device at index", i, ":", err.Error())
				return err
			}
			g.tags["pci_identifier"] = fmt.Sprintf(
				"%08X:%02X:%02X.0",
				pciInfo.Domain,
				pciInfo.Bus,
				pciInfo.Device)
		}
	}
	m.statsProcessedMetrics = 0
	m.init = true
	return nil
}

func (m *NvidiaCollector) Read(interval time.Duration, output chan lp.CCMetric) {
	if !m.init {
		return
	}

	for i := range m.gpus {
		device := &m.gpus[i]

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
					y, err := lp.New("nv_util", device.tags, m.meta, map[string]interface{}{"value": float64(util.Gpu)}, time.Now())
					if err == nil {
						y.AddMeta("unit", "%")
						output <- y
						m.statsProcessedMetrics++
					}
				}
				if !device.excludeMetrics["nv_mem_util"] {
					y, err := lp.New("nv_mem_util", device.tags, m.meta, map[string]interface{}{"value": float64(util.Memory)}, time.Now())
					if err == nil {
						y.AddMeta("unit", "%")
						output <- y
						m.statsProcessedMetrics++
					}
				}
			}
		}

		if !device.excludeMetrics["nv_mem_total"] || !device.excludeMetrics["nv_fb_memory"] {
			// Retrieves the amount of used, free and total memory available on the device, in bytes.
			//
			// Enabling ECC reduces the amount of total available memory, due to the extra required parity bits.
			//
			// The reported amount of used memory is equal to the sum of memory allocated by all active channels on the device.
			//
			// Available memory info:
			// * Free: Unallocated FB memory (in bytes).
			// * Total: Total installed FB memory (in bytes).
			// * Used: Allocated FB memory (in bytes). Note that the driver/GPU always sets aside a small amount of memory for bookkeeping.
			//
			// Note:
			// In MIG mode, if device handle is provided, the API returns aggregate information, only if the caller has appropriate privileges.
			// Per-instance information can be queried by using specific MIG device handles.
			meminfo, ret := nvml.DeviceGetMemoryInfo(device.device)
			if ret == nvml.SUCCESS {
				if !device.excludeMetrics["nv_mem_total"] {
					t := float64(meminfo.Total) / (1024 * 1024)
					y, err := lp.New("nv_mem_total", device.tags, m.meta, map[string]interface{}{"value": t}, time.Now())
					if err == nil {
						y.AddMeta("unit", "MByte")
						output <- y
						m.statsProcessedMetrics++
					}
				}

				if !device.excludeMetrics["nv_fb_memory"] {
					f := float64(meminfo.Used) / (1024 * 1024)
					y, err := lp.New("nv_fb_memory", device.tags, m.meta, map[string]interface{}{"value": f}, time.Now())
					if err == nil {
						y.AddMeta("unit", "MByte")
						output <- y
						m.statsProcessedMetrics++
					}
				}
			}
		}

		if !device.excludeMetrics["nv_temp"] {
			// Retrieves the current temperature readings for the device, in degrees C.
			//
			// Available temperature sensors:
			// * TEMPERATURE_GPU: Temperature sensor for the GPU die.
			// * NVML_TEMPERATURE_COUNT
			temp, ret := nvml.DeviceGetTemperature(device.device, nvml.TEMPERATURE_GPU)
			if ret == nvml.SUCCESS {
				y, err := lp.New("nv_temp", device.tags, m.meta, map[string]interface{}{"value": float64(temp)}, time.Now())
				if err == nil {
					y.AddMeta("unit", "degC")
					output <- y
					m.statsProcessedMetrics++
				}
			}
		}

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
				y, err := lp.New("nv_fan", device.tags, m.meta, map[string]interface{}{"value": float64(fan)}, time.Now())
				if err == nil {
					y.AddMeta("unit", "%")
					output <- y
					m.statsProcessedMetrics++
				}
			}
		}

		if !device.excludeMetrics["nv_ecc_mode"] {
			// Retrieves the current and pending ECC modes for the device.
			//
			// For Fermi or newer fully supported devices. Only applicable to devices with ECC.
			// Requires NVML_INFOROM_ECC version 1.0 or higher.
			//
			// Changing ECC modes requires a reboot.
			// The "pending" ECC mode refers to the target mode following the next reboot.
			_, ecc_pend, ret := nvml.DeviceGetEccMode(device.device)
			if ret == nvml.SUCCESS {
				var y lp.CCMetric
				var err error
				switch ecc_pend {
				case nvml.FEATURE_DISABLED:
					y, err = lp.New("nv_ecc_mode", device.tags, m.meta, map[string]interface{}{"value": "OFF"}, time.Now())
				case nvml.FEATURE_ENABLED:
					y, err = lp.New("nv_ecc_mode", device.tags, m.meta, map[string]interface{}{"value": "ON"}, time.Now())
				default:
					y, err = lp.New("nv_ecc_mode", device.tags, m.meta, map[string]interface{}{"value": "UNKNOWN"}, time.Now())
				}
				if err == nil {
					output <- y
					m.statsProcessedMetrics++
				}
			} else if ret == nvml.ERROR_NOT_SUPPORTED {
				y, err := lp.New("nv_ecc_mode", device.tags, m.meta, map[string]interface{}{"value": "N/A"}, time.Now())
				if err == nil {
					output <- y
					m.statsProcessedMetrics++
				}
			}
		}

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
				y, err := lp.New("nv_perf_state", device.tags, m.meta, map[string]interface{}{"value": fmt.Sprintf("P%d", int(pState))}, time.Now())
				if err == nil {
					output <- y
					m.statsProcessedMetrics++
				}
			}
		}

		if !device.excludeMetrics["nv_power_usage_report"] {
			// Retrieves power usage for this GPU in milliwatts and its associated circuitry (e.g. memory)
			//
			// On Fermi and Kepler GPUs the reading is accurate to within +/- 5% of current power draw.
			//
			// It is only available if power management mode is supported
			power, ret := nvml.DeviceGetPowerUsage(device.device)
			if ret == nvml.SUCCESS {
				y, err := lp.New("nv_power_usage_report", device.tags, m.meta, map[string]interface{}{"value": float64(power) / 1000}, time.Now())
				if err == nil {
					y.AddMeta("unit", "watts")
					output <- y
					m.statsProcessedMetrics++
				}
			}
		}

		// Retrieves the current clock speeds for the device.
		//
		// Available clock information:
		// * CLOCK_GRAPHICS: Graphics clock domain.
		// * CLOCK_SM: Streaming Multiprocessor clock domain.
		// * CLOCK_MEM: Memory clock domain.
		if !device.excludeMetrics["nv_graphics_clock_report"] {
			graphicsClock, ret := nvml.DeviceGetClockInfo(device.device, nvml.CLOCK_GRAPHICS)
			if ret == nvml.SUCCESS {
				y, err := lp.New("nv_graphics_clock_report", device.tags, m.meta, map[string]interface{}{"value": float64(graphicsClock)}, time.Now())
				if err == nil {
					y.AddMeta("unit", "MHz")
					output <- y
					m.statsProcessedMetrics++
				}
			}
		}

		if !device.excludeMetrics["nv_sm_clock_report"] {
			smCock, ret := nvml.DeviceGetClockInfo(device.device, nvml.CLOCK_SM)
			if ret == nvml.SUCCESS {
				y, err := lp.New("nv_sm_clock_report", device.tags, m.meta, map[string]interface{}{"value": float64(smCock)}, time.Now())
				if err == nil {
					y.AddMeta("unit", "MHz")
					output <- y
					m.statsProcessedMetrics++
				}
			}
		}

		if !device.excludeMetrics["nv_mem_clock_report"] {
			memClock, ret := nvml.DeviceGetClockInfo(device.device, nvml.CLOCK_MEM)
			if ret == nvml.SUCCESS {
				y, err := lp.New("nv_mem_clock_report", device.tags, m.meta, map[string]interface{}{"value": float64(memClock)}, time.Now())
				if err == nil {
					y.AddMeta("unit", "MHz")
					output <- y
					m.statsProcessedMetrics++
				}
			}
		}

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
				y, err := lp.New("nv_max_graphics_clock", device.tags, m.meta, map[string]interface{}{"value": float64(max_gclk)}, time.Now())
				if err == nil {
					y.AddMeta("unit", "MHz")
					output <- y
					m.statsProcessedMetrics++
				}
			}
		}

		if !device.excludeMetrics["nv_max_sm_clock"] {
			maxSmClock, ret := nvml.DeviceGetClockInfo(device.device, nvml.CLOCK_SM)
			if ret == nvml.SUCCESS {
				y, err := lp.New("nv_max_sm_clock", device.tags, m.meta, map[string]interface{}{"value": float64(maxSmClock)}, time.Now())
				if err == nil {
					y.AddMeta("unit", "MHz")
					output <- y
					m.statsProcessedMetrics++
				}
			}
		}

		if !device.excludeMetrics["nv_max_mem_clock"] {
			maxMemClock, ret := nvml.DeviceGetClockInfo(device.device, nvml.CLOCK_MEM)
			if ret == nvml.SUCCESS {
				y, err := lp.New("nv_max_mem_clock", device.tags, m.meta, map[string]interface{}{"value": float64(maxMemClock)}, time.Now())
				if err == nil {
					y.AddMeta("unit", "MHz")
					output <- y
					m.statsProcessedMetrics++
				}
			}
		}

		if !device.excludeMetrics["nv_ecc_db_error"] {
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
				y, err := lp.New("nv_ecc_db_error", device.tags, m.meta, map[string]interface{}{"value": float64(ecc_db)}, time.Now())
				if err == nil {
					output <- y
					m.statsProcessedMetrics++
				}
			}
		}

		if !device.excludeMetrics["nv_ecc_sb_error"] {
			ecc_sb, ret := nvml.DeviceGetTotalEccErrors(device.device, nvml.MEMORY_ERROR_TYPE_CORRECTED, nvml.AGGREGATE_ECC)
			if ret == nvml.SUCCESS {
				y, err := lp.New("nv_ecc_sb_error", device.tags, m.meta, map[string]interface{}{"value": float64(ecc_sb)}, time.Now())
				if err == nil {
					output <- y
					m.statsProcessedMetrics++
				}
			}
		}

		if !device.excludeMetrics["nv_power_man_limit"] {
			// Retrieves the power management limit associated with this device.
			//
			// For Fermi or newer fully supported devices.
			//
			// The power limit defines the upper boundary for the card's power draw.
			// If the card's total power draw reaches this limit the power management algorithm kicks in.
			pwr_limit, ret := nvml.DeviceGetPowerManagementLimit(device.device)
			if ret == nvml.SUCCESS {
				y, err := lp.New("nv_power_man_limit", device.tags, m.meta, map[string]interface{}{"value": float64(pwr_limit) / 1000}, time.Now())
				if err == nil {
					y.AddMeta("unit", "watts")
					output <- y
					m.statsProcessedMetrics++
				}
			}
		}

		if !device.excludeMetrics["nv_encoder_util"] {
			// Retrieves the current utilization and sampling size in microseconds for the Encoder
			//
			// For Kepler or newer fully supported devices.
			//
			// Note: On MIG-enabled GPUs, querying encoder utilization is not currently supported.
			enc_util, _, ret := nvml.DeviceGetEncoderUtilization(device.device)
			if ret == nvml.SUCCESS {
				y, err := lp.New("nv_encoder_util", device.tags, m.meta, map[string]interface{}{"value": float64(enc_util)}, time.Now())
				if err == nil {
					y.AddMeta("unit", "%")
					output <- y
					m.statsProcessedMetrics++
				}
			}
		}

		if !device.excludeMetrics["nv_decoder_util"] {
			// Retrieves the current utilization and sampling size in microseconds for the Decoder
			//
			// For Kepler or newer fully supported devices.
			//
			// Note: On MIG-enabled GPUs, querying decoder utilization is not currently supported.
			dec_util, _, ret := nvml.DeviceGetDecoderUtilization(device.device)
			if ret == nvml.SUCCESS {
				y, err := lp.New("nv_decoder_util", device.tags, m.meta, map[string]interface{}{"value": float64(dec_util)}, time.Now())
				if err == nil {
					y.AddMeta("unit", "%")
					output <- y
					m.statsProcessedMetrics++
				}
			}
		}
	}
	stats.ComponentStatInt(m.name, "collected_metrics", m.statsProcessedMetrics)
}

func (m *NvidiaCollector) Close() {
	if m.init {
		nvml.Shutdown()
		m.init = false
	}
}
