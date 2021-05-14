package collectors

import (
	"errors"
	"fmt"
	"github.com/NVIDIA/go-nvml/pkg/nvml"
	"log"
	"time"
)

type NvidiaCollector struct {
	MetricCollector
	num_gpus int
}

func (m *NvidiaCollector) Init() error {
	m.name = "NvidiaCollector"
	m.setup()
	m.num_gpus = 0
	ret := nvml.Init()
	if ret != nvml.SUCCESS {
		err := errors.New(nvml.ErrorString(ret))
		log.Print(err)
		return err
	}
	m.num_gpus, ret = nvml.DeviceGetCount()
	if ret != nvml.SUCCESS {
		err := errors.New(nvml.ErrorString(ret))
		return err
	}
	return nil
}

func (m *NvidiaCollector) Read(interval time.Duration) {

	for i := 0; i < m.num_gpus; i++ {
		device, ret := nvml.DeviceGetHandleByIndex(i)
		if ret != nvml.SUCCESS {
			log.Fatalf("Unable to get device at index %d: %v", i, nvml.ErrorString(ret))
			return
		}
		base := fmt.Sprintf("gpu%d", i)

		util, ret := nvml.DeviceGetUtilizationRates(device)
		if ret == nvml.SUCCESS {
			m.node[fmt.Sprintf("%s_util", base)] = float64(util.Gpu)
			m.node[fmt.Sprintf("%s_mem_util", base)] = float64(util.Memory)
		}

		meminfo, ret := nvml.DeviceGetMemoryInfo(device)
		if ret == nvml.SUCCESS {
			m.node[fmt.Sprintf("%s_mem_total", base)] = float64(meminfo.Total) / (1024 * 1024)
			m.node[fmt.Sprintf("%s_fb_memory", base)] = float64(meminfo.Used) / (1024 * 1024)
		}

		temp, ret := nvml.DeviceGetTemperature(device, nvml.TEMPERATURE_GPU)
		if ret == nvml.SUCCESS {
			m.node[fmt.Sprintf("%s_temp", base)] = float64(temp)
		}

		fan, ret := nvml.DeviceGetFanSpeed(device)
		if ret == nvml.SUCCESS {
			m.node[fmt.Sprintf("%s_fan", base)] = float64(fan)
		}

		_, ecc_pend, ret := nvml.DeviceGetEccMode(device)
		if ret == nvml.SUCCESS {
			switch ecc_pend {
			case nvml.FEATURE_DISABLED:
				m.node[fmt.Sprintf("%s_ecc_mode", base)] = string("OFF")
			case nvml.FEATURE_ENABLED:
				m.node[fmt.Sprintf("%s_ecc_mode", base)] = string("ON")
			default:
				m.node[fmt.Sprintf("%s_ecc_mode", base)] = string("UNKNOWN")
			}
		} else if ret == nvml.ERROR_NOT_SUPPORTED {
			m.node[fmt.Sprintf("%s_ecc_mode", base)] = string("N/A")
		}

		pstate, ret := nvml.DeviceGetPerformanceState(device)
		if ret == nvml.SUCCESS {
			m.node[fmt.Sprintf("%s_perf_state", base)] = fmt.Sprintf("P%d", int(pstate))
		}

		power, ret := nvml.DeviceGetPowerUsage(device)
		if ret == nvml.SUCCESS {
			m.node[fmt.Sprintf("%s_power_usage_report", base)] = float64(power) / 1000
		}

		gclk, ret := nvml.DeviceGetClockInfo(device, nvml.CLOCK_GRAPHICS)
		if ret == nvml.SUCCESS {
			m.node[fmt.Sprintf("%s_graphics_clock_report", base)] = float64(gclk)
		}

		smclk, ret := nvml.DeviceGetClockInfo(device, nvml.CLOCK_SM)
		if ret == nvml.SUCCESS {
			m.node[fmt.Sprintf("%s_sm_clock_report", base)] = float64(smclk)
		}

		memclk, ret := nvml.DeviceGetClockInfo(device, nvml.CLOCK_MEM)
		if ret == nvml.SUCCESS {
			m.node[fmt.Sprintf("%s_mem_clock_report", base)] = float64(memclk)
		}

		max_gclk, ret := nvml.DeviceGetMaxClockInfo(device, nvml.CLOCK_GRAPHICS)
		if ret == nvml.SUCCESS {
			m.node[fmt.Sprintf("%s_max_graphics_clock", base)] = float64(max_gclk)
		}

		max_smclk, ret := nvml.DeviceGetClockInfo(device, nvml.CLOCK_SM)
		if ret == nvml.SUCCESS {
			m.node[fmt.Sprintf("%s_max_sm_clock", base)] = float64(max_smclk)
		}

		max_memclk, ret := nvml.DeviceGetClockInfo(device, nvml.CLOCK_MEM)
		if ret == nvml.SUCCESS {
			m.node[fmt.Sprintf("%s_max_mem_clock", base)] = float64(max_memclk)
		}

		ecc_db, ret := nvml.DeviceGetTotalEccErrors(device, 1, 1)
		if ret == nvml.SUCCESS {
			m.node[fmt.Sprintf("%s_ecc_db_error", base)] = float64(ecc_db)
		}

		ecc_sb, ret := nvml.DeviceGetTotalEccErrors(device, 0, 1)
		if ret == nvml.SUCCESS {
			m.node[fmt.Sprintf("%s_ecc_sb_error", base)] = float64(ecc_sb)
		}

		pwr_limit, ret := nvml.DeviceGetPowerManagementLimit(device)
		if ret == nvml.SUCCESS {
			m.node[fmt.Sprintf("%s_power_man_limit", base)] = float64(pwr_limit)
		}

		enc_util, _, ret := nvml.DeviceGetEncoderUtilization(device)
		if ret == nvml.SUCCESS {
			m.node[fmt.Sprintf("%s_power_man_limit", base)] = float64(enc_util)
		}

		dec_util, _, ret := nvml.DeviceGetDecoderUtilization(device)
		if ret == nvml.SUCCESS {
			m.node[fmt.Sprintf("%s_power_man_limit", base)] = float64(dec_util)
		}
	}

}

func (m *NvidiaCollector) Close() {
	nvml.Shutdown()
	return
}
