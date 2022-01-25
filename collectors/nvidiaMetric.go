package collectors

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"
	lp "github.com/ClusterCockpit/cc-metric-collector/internal/ccMetric"
	"github.com/NVIDIA/go-nvml/pkg/nvml"
)

type NvidiaCollectorConfig struct {
	ExcludeMetrics []string `json:"exclude_metrics,omitempty"`
	ExcludeDevices []string `json:"exclude_devices,omitempty"`
}

type NvidiaCollector struct {
	metricCollector
	num_gpus int
	config   NvidiaCollectorConfig
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
	m.setup()
	m.meta = map[string]string{"source": m.name, "group": "Nvidia"}
	if len(config) > 0 {
		err = json.Unmarshal(config, &m.config)
		if err != nil {
			return err
		}
	}
	m.num_gpus = 0
	defer m.CatchPanic()
	ret := nvml.Init()
	if ret != nvml.SUCCESS {
		err = errors.New(nvml.ErrorString(ret))
		return err
	}
	m.num_gpus, ret = nvml.DeviceGetCount()
	if ret != nvml.SUCCESS {
		err = errors.New(nvml.ErrorString(ret))
		return err
	}
	m.init = true
	return nil
}

func (m *NvidiaCollector) Read(interval time.Duration, output chan lp.CCMetric) {
	if !m.init {
		return
	}
	for i := 0; i < m.num_gpus; i++ {
		device, ret := nvml.DeviceGetHandleByIndex(i)
		if ret != nvml.SUCCESS {
			log.Fatalf("Unable to get device at index %d: %v", i, nvml.ErrorString(ret))
			return
		}
		_, skip := stringArrayContains(m.config.ExcludeDevices, fmt.Sprintf("%d", i))
		if skip {
			continue
		}
		tags := map[string]string{"type": "accelerator", "type-id": fmt.Sprintf("%d", i)}

		util, ret := nvml.DeviceGetUtilizationRates(device)
		if ret == nvml.SUCCESS {
			_, skip = stringArrayContains(m.config.ExcludeMetrics, "util")
			y, err := lp.New("util", tags, m.meta, map[string]interface{}{"value": float64(util.Gpu)}, time.Now())
			if err == nil && !skip {
				output <- y
			}
			_, skip = stringArrayContains(m.config.ExcludeMetrics, "mem_util")
			y, err = lp.New("mem_util", tags, m.meta, map[string]interface{}{"value": float64(util.Memory)}, time.Now())
			if err == nil && !skip {
				output <- y
			}
		}

		meminfo, ret := nvml.DeviceGetMemoryInfo(device)
		if ret == nvml.SUCCESS {
			t := float64(meminfo.Total) / (1024 * 1024)
			_, skip = stringArrayContains(m.config.ExcludeMetrics, "mem_total")
			y, err := lp.New("mem_total", tags, m.meta, map[string]interface{}{"value": t}, time.Now())
			if err == nil && !skip {
				y.AddMeta("unit", "MByte")
				output <- y
			}
			f := float64(meminfo.Used) / (1024 * 1024)
			_, skip = stringArrayContains(m.config.ExcludeMetrics, "fb_memory")
			y, err = lp.New("fb_memory", tags, m.meta, map[string]interface{}{"value": f}, time.Now())
			if err == nil && !skip {
				y.AddMeta("unit", "MByte")
				output <- y
			}
		}

		temp, ret := nvml.DeviceGetTemperature(device, nvml.TEMPERATURE_GPU)
		if ret == nvml.SUCCESS {
			_, skip = stringArrayContains(m.config.ExcludeMetrics, "temp")
			y, err := lp.New("temp", tags, m.meta, map[string]interface{}{"value": float64(temp)}, time.Now())
			if err == nil && !skip {
				y.AddMeta("unit", "degC")
				output <- y
			}
		}

		fan, ret := nvml.DeviceGetFanSpeed(device)
		if ret == nvml.SUCCESS {
			_, skip = stringArrayContains(m.config.ExcludeMetrics, "fan")
			y, err := lp.New("fan", tags, m.meta, map[string]interface{}{"value": float64(fan)}, time.Now())
			if err == nil && !skip {
				output <- y
			}
		}

		_, ecc_pend, ret := nvml.DeviceGetEccMode(device)
		if ret == nvml.SUCCESS {
			var y lp.CCMetric
			var err error
			switch ecc_pend {
			case nvml.FEATURE_DISABLED:
				y, err = lp.New("ecc_mode", tags, m.meta, map[string]interface{}{"value": string("OFF")}, time.Now())
			case nvml.FEATURE_ENABLED:
				y, err = lp.New("ecc_mode", tags, m.meta, map[string]interface{}{"value": string("ON")}, time.Now())
			default:
				y, err = lp.New("ecc_mode", tags, m.meta, map[string]interface{}{"value": string("UNKNOWN")}, time.Now())
			}
			_, skip = stringArrayContains(m.config.ExcludeMetrics, "ecc_mode")
			if err == nil && !skip {
				output <- y
			}
		} else if ret == nvml.ERROR_NOT_SUPPORTED {
			_, skip = stringArrayContains(m.config.ExcludeMetrics, "ecc_mode")
			y, err := lp.New("ecc_mode", tags, m.meta, map[string]interface{}{"value": string("N/A")}, time.Now())
			if err == nil && !skip {
				output <- y
			}
		}

		pstate, ret := nvml.DeviceGetPerformanceState(device)
		if ret == nvml.SUCCESS {
			_, skip = stringArrayContains(m.config.ExcludeMetrics, "perf_state")
			y, err := lp.New("perf_state", tags, m.meta, map[string]interface{}{"value": fmt.Sprintf("P%d", int(pstate))}, time.Now())
			if err == nil && !skip {
				output <- y
			}
		}

		power, ret := nvml.DeviceGetPowerUsage(device)
		if ret == nvml.SUCCESS {
			_, skip = stringArrayContains(m.config.ExcludeMetrics, "power_usage_report")
			y, err := lp.New("power_usage_report", tags, m.meta, map[string]interface{}{"value": float64(power) / 1000}, time.Now())
			if err == nil && !skip {
				output <- y
			}
		}

		gclk, ret := nvml.DeviceGetClockInfo(device, nvml.CLOCK_GRAPHICS)
		if ret == nvml.SUCCESS {
			_, skip = stringArrayContains(m.config.ExcludeMetrics, "graphics_clock_report")
			y, err := lp.New("graphics_clock_report", tags, m.meta, map[string]interface{}{"value": float64(gclk)}, time.Now())
			if err == nil && !skip {
				output <- y
			}
		}

		smclk, ret := nvml.DeviceGetClockInfo(device, nvml.CLOCK_SM)
		if ret == nvml.SUCCESS {
			_, skip = stringArrayContains(m.config.ExcludeMetrics, "sm_clock_report")
			y, err := lp.New("sm_clock_report", tags, m.meta, map[string]interface{}{"value": float64(smclk)}, time.Now())
			if err == nil && !skip {
				output <- y
			}
		}

		memclk, ret := nvml.DeviceGetClockInfo(device, nvml.CLOCK_MEM)
		if ret == nvml.SUCCESS {
			_, skip = stringArrayContains(m.config.ExcludeMetrics, "mem_clock_report")
			y, err := lp.New("mem_clock_report", tags, m.meta, map[string]interface{}{"value": float64(memclk)}, time.Now())
			if err == nil && !skip {
				output <- y
			}
		}

		max_gclk, ret := nvml.DeviceGetMaxClockInfo(device, nvml.CLOCK_GRAPHICS)
		if ret == nvml.SUCCESS {
			_, skip = stringArrayContains(m.config.ExcludeMetrics, "max_graphics_clock")
			y, err := lp.New("max_graphics_clock", tags, m.meta, map[string]interface{}{"value": float64(max_gclk)}, time.Now())
			if err == nil && !skip {
				output <- y
			}
		}

		max_smclk, ret := nvml.DeviceGetClockInfo(device, nvml.CLOCK_SM)
		if ret == nvml.SUCCESS {
			_, skip = stringArrayContains(m.config.ExcludeMetrics, "max_sm_clock")
			y, err := lp.New("max_sm_clock", tags, m.meta, map[string]interface{}{"value": float64(max_smclk)}, time.Now())
			if err == nil && !skip {
				output <- y
			}
		}

		max_memclk, ret := nvml.DeviceGetClockInfo(device, nvml.CLOCK_MEM)
		if ret == nvml.SUCCESS {
			_, skip = stringArrayContains(m.config.ExcludeMetrics, "max_mem_clock")
			y, err := lp.New("max_mem_clock", tags, m.meta, map[string]interface{}{"value": float64(max_memclk)}, time.Now())
			if err == nil && !skip {
				output <- y
			}
		}

		ecc_db, ret := nvml.DeviceGetTotalEccErrors(device, 1, 1)
		if ret == nvml.SUCCESS {
			_, skip = stringArrayContains(m.config.ExcludeMetrics, "ecc_db_error")
			y, err := lp.New("ecc_db_error", tags, m.meta, map[string]interface{}{"value": float64(ecc_db)}, time.Now())
			if err == nil && !skip {
				output <- y
			}
		}

		ecc_sb, ret := nvml.DeviceGetTotalEccErrors(device, 0, 1)
		if ret == nvml.SUCCESS {
			_, skip = stringArrayContains(m.config.ExcludeMetrics, "ecc_sb_error")
			y, err := lp.New("ecc_sb_error", tags, m.meta, map[string]interface{}{"value": float64(ecc_sb)}, time.Now())
			if err == nil && !skip {
				output <- y
			}
		}

		pwr_limit, ret := nvml.DeviceGetPowerManagementLimit(device)
		if ret == nvml.SUCCESS {
			_, skip = stringArrayContains(m.config.ExcludeMetrics, "power_man_limit")
			y, err := lp.New("power_man_limit", tags, m.meta, map[string]interface{}{"value": float64(pwr_limit)}, time.Now())
			if err == nil && !skip {
				output <- y
			}
		}

		enc_util, _, ret := nvml.DeviceGetEncoderUtilization(device)
		if ret == nvml.SUCCESS {
			_, skip = stringArrayContains(m.config.ExcludeMetrics, "encoder_util")
			y, err := lp.New("encoder_util", tags, m.meta, map[string]interface{}{"value": float64(enc_util)}, time.Now())
			if err == nil && !skip {
				output <- y
			}
		}

		dec_util, _, ret := nvml.DeviceGetDecoderUtilization(device)
		if ret == nvml.SUCCESS {
			_, skip = stringArrayContains(m.config.ExcludeMetrics, "decoder_util")
			y, err := lp.New("decoder_util", tags, m.meta, map[string]interface{}{"value": float64(dec_util)}, time.Now())
			if err == nil && !skip {
				output <- y
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
