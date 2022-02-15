package collectors

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	cclog "github.com/ClusterCockpit/cc-metric-collector/internal/ccLogger"
	lp "github.com/ClusterCockpit/cc-metric-collector/internal/ccMetric"
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
	num_gpus int
	config   NvidiaCollectorConfig
	gpus     []NvidiaCollectorDevice
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

	m.init = true
	return nil
}

func (m *NvidiaCollector) Read(interval time.Duration, output chan lp.CCMetric) {
	if !m.init {
		return
	}

	for _, device := range m.gpus {

		exclude := func(metric string) bool {
			if _, ok := device.excludeMetrics[metric]; !ok {
				return true
			}
			return false
		}

		ex_nv_util := exclude("nv_util")
		ex_nv_mem_util := exclude("nv_mem_util")
		if (!ex_nv_util) || (!ex_nv_mem_util) {
			util, ret := nvml.DeviceGetUtilizationRates(device.device)
			if ret == nvml.SUCCESS {
				if !ex_nv_util {
					y, err := lp.New("nv_util", device.tags, m.meta, map[string]interface{}{"value": float64(util.Gpu)}, time.Now())
					if err == nil {
						output <- y
					}
				}
				if !ex_nv_mem_util {
					y, err := lp.New("nv_mem_util", device.tags, m.meta, map[string]interface{}{"value": float64(util.Memory)}, time.Now())
					if err == nil {
						output <- y
					}
				}
			}
		}

		ex_nv_mem_total := exclude("nv_mem_total")
		ex_nv_fb_memory := exclude("nv_fb_memory")
		if (!ex_nv_mem_total) || (!ex_nv_fb_memory) {
			meminfo, ret := nvml.DeviceGetMemoryInfo(device.device)
			if ret == nvml.SUCCESS {
				if !ex_nv_mem_total {
					t := float64(meminfo.Total) / (1024 * 1024)
					y, err := lp.New("nv_mem_total", device.tags, m.meta, map[string]interface{}{"value": t}, time.Now())
					if err == nil {
						y.AddMeta("unit", "MByte")
						output <- y
					}
				}

				if !ex_nv_fb_memory {
					f := float64(meminfo.Used) / (1024 * 1024)
					y, err := lp.New("nv_fb_memory", device.tags, m.meta, map[string]interface{}{"value": f}, time.Now())
					if err == nil {
						y.AddMeta("unit", "MByte")
						output <- y
					}
				}
			}
		}

		if !exclude("nv_temp") {
			temp, ret := nvml.DeviceGetTemperature(device.device, nvml.TEMPERATURE_GPU)
			if ret == nvml.SUCCESS {
				y, err := lp.New("nv_temp", device.tags, m.meta, map[string]interface{}{"value": float64(temp)}, time.Now())
				if err == nil {
					y.AddMeta("unit", "degC")
					output <- y
				}
			}
		}

		if !exclude("nv_fan") {
			fan, ret := nvml.DeviceGetFanSpeed(device.device)
			if ret == nvml.SUCCESS {
				y, err := lp.New("nv_fan", device.tags, m.meta, map[string]interface{}{"value": float64(fan)}, time.Now())
				if err == nil {
					output <- y
				}
			}
		}

		if !exclude("nv_ecc_mode") {
			_, ecc_pend, ret := nvml.DeviceGetEccMode(device.device)
			if ret == nvml.SUCCESS {
				var y lp.CCMetric
				var err error
				switch ecc_pend {
				case nvml.FEATURE_DISABLED:
					y, err = lp.New("nv_ecc_mode", device.tags, m.meta, map[string]interface{}{"value": string("OFF")}, time.Now())
				case nvml.FEATURE_ENABLED:
					y, err = lp.New("nv_ecc_mode", device.tags, m.meta, map[string]interface{}{"value": string("ON")}, time.Now())
				default:
					y, err = lp.New("nv_ecc_mode", device.tags, m.meta, map[string]interface{}{"value": string("UNKNOWN")}, time.Now())
				}
				if err == nil {
					output <- y
				}
			} else if ret == nvml.ERROR_NOT_SUPPORTED {
				y, err := lp.New("nv_ecc_mode", device.tags, m.meta, map[string]interface{}{"value": string("N/A")}, time.Now())
				if err == nil {
					output <- y
				}
			}
		}

		if !exclude("nv_perf_state") {
			pstate, ret := nvml.DeviceGetPerformanceState(device.device)
			if ret == nvml.SUCCESS {
				y, err := lp.New("nv_perf_state", device.tags, m.meta, map[string]interface{}{"value": fmt.Sprintf("P%d", int(pstate))}, time.Now())
				if err == nil {
					output <- y
				}
			}
		}

		if !exclude("nv_power_usage_report") {
			power, ret := nvml.DeviceGetPowerUsage(device.device)
			if ret == nvml.SUCCESS {
				y, err := lp.New("nv_power_usage_report", device.tags, m.meta, map[string]interface{}{"value": float64(power) / 1000}, time.Now())
				if err == nil {
					output <- y
				}
			}
		}

		if !exclude("nv_graphics_clock_report") {
			gclk, ret := nvml.DeviceGetClockInfo(device.device, nvml.CLOCK_GRAPHICS)
			if ret == nvml.SUCCESS {
				y, err := lp.New("nv_graphics_clock_report", device.tags, m.meta, map[string]interface{}{"value": float64(gclk)}, time.Now())
				if err == nil {
					output <- y
				}
			}
		}

		if !exclude("nv_sm_clock_report") {
			smclk, ret := nvml.DeviceGetClockInfo(device.device, nvml.CLOCK_SM)
			if ret == nvml.SUCCESS {
				y, err := lp.New("nv_sm_clock_report", device.tags, m.meta, map[string]interface{}{"value": float64(smclk)}, time.Now())
				if err == nil {
					output <- y
				}
			}
		}

		if !exclude("nv_mem_clock_report") {
			memclk, ret := nvml.DeviceGetClockInfo(device.device, nvml.CLOCK_MEM)
			if ret == nvml.SUCCESS {
				y, err := lp.New("nv_mem_clock_report", device.tags, m.meta, map[string]interface{}{"value": float64(memclk)}, time.Now())
				if err == nil {
					output <- y
				}
			}
		}

		if !exclude("nv_max_graphics_clock") {
			max_gclk, ret := nvml.DeviceGetMaxClockInfo(device.device, nvml.CLOCK_GRAPHICS)
			if ret == nvml.SUCCESS {
				y, err := lp.New("nv_max_graphics_clock", device.tags, m.meta, map[string]interface{}{"value": float64(max_gclk)}, time.Now())
				if err == nil {
					output <- y
				}
			}
		}

		if !exclude("nv_max_sm_clock") {
			max_smclk, ret := nvml.DeviceGetClockInfo(device.device, nvml.CLOCK_SM)
			if ret == nvml.SUCCESS {
				y, err := lp.New("nv_max_sm_clock", device.tags, m.meta, map[string]interface{}{"value": float64(max_smclk)}, time.Now())
				if err == nil {
					output <- y
				}
			}
		}

		if !exclude("nv_max_mem_clock") {
			max_memclk, ret := nvml.DeviceGetClockInfo(device.device, nvml.CLOCK_MEM)
			if ret == nvml.SUCCESS {
				y, err := lp.New("nv_max_mem_clock", device.tags, m.meta, map[string]interface{}{"value": float64(max_memclk)}, time.Now())
				if err == nil {
					output <- y
				}
			}
		}

		if !exclude("nv_ecc_db_error") {
			ecc_db, ret := nvml.DeviceGetTotalEccErrors(device.device, 1, 1)
			if ret == nvml.SUCCESS {
				y, err := lp.New("nv_ecc_db_error", device.tags, m.meta, map[string]interface{}{"value": float64(ecc_db)}, time.Now())
				if err == nil {
					output <- y
				}
			}
		}

		if !exclude("nv_ecc_sb_error") {
			ecc_sb, ret := nvml.DeviceGetTotalEccErrors(device.device, 0, 1)
			if ret == nvml.SUCCESS {
				y, err := lp.New("nv_ecc_sb_error", device.tags, m.meta, map[string]interface{}{"value": float64(ecc_sb)}, time.Now())
				if err == nil {
					output <- y
				}
			}
		}

		if !exclude("nv_power_man_limit") {
			pwr_limit, ret := nvml.DeviceGetPowerManagementLimit(device.device)
			if ret == nvml.SUCCESS {
				y, err := lp.New("nv_power_man_limit", device.tags, m.meta, map[string]interface{}{"value": float64(pwr_limit)}, time.Now())
				if err == nil {
					output <- y
				}
			}
		}

		if !exclude("nv_encoder_util") {
			enc_util, _, ret := nvml.DeviceGetEncoderUtilization(device.device)
			if ret == nvml.SUCCESS {
				y, err := lp.New("nv_encoder_util", device.tags, m.meta, map[string]interface{}{"value": float64(enc_util)}, time.Now())
				if err == nil {
					output <- y
				}
			}
		}

		if !exclude("nv_decoder_util") {
			dec_util, _, ret := nvml.DeviceGetDecoderUtilization(device.device)
			if ret == nvml.SUCCESS {
				y, err := lp.New("nv_decoder_util", device.tags, m.meta, map[string]interface{}{"value": float64(dec_util)}, time.Now())
				if err == nil {
					output <- y
				}
			}
		}
	}

}

func (m *NvidiaCollector) Close() {
	if m.init {
		nvml.Shutdown()
		m.init = false
	}
}
