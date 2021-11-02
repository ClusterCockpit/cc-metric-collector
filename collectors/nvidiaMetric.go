package collectors

import (
	"errors"
	"fmt"
	"github.com/NVIDIA/go-nvml/pkg/nvml"
	lp "github.com/influxdata/line-protocol"
	"log"
	"time"
)

type NvidiaCollector struct {
	MetricCollector
	num_gpus int
}

func (m *NvidiaCollector) CatchPanic() error {

	if rerr := recover(); rerr != nil {
		log.Print("CatchPanic ", string(rerr.(string)))
		err := errors.New(rerr.(string))
		return err
	}
	return nil
}

func (m *NvidiaCollector) Init() error {
	m.name = "NvidiaCollector"
	m.setup()
	m.num_gpus = 0
	defer m.CatchPanic()
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
	m.init = true
	return nil
}

func (m *NvidiaCollector) Read(interval time.Duration, out *[]lp.MutableMetric) {

	for i := 0; i < m.num_gpus; i++ {
		device, ret := nvml.DeviceGetHandleByIndex(i)
		if ret != nvml.SUCCESS {
			log.Fatalf("Unable to get device at index %d: %v", i, nvml.ErrorString(ret))
			return
		}
		tags := map[string]string{"type": "accelerator", "type-id": fmt.Sprintf("%d", i)}

		util, ret := nvml.DeviceGetUtilizationRates(device)
		if ret == nvml.SUCCESS {
			y, err := lp.New("util", tags, map[string]interface{}{"value": float64(util.Gpu)}, time.Now())
			if err == nil {
				*out = append(*out, y)
			}
			y, err = lp.New("mem_util", tags, map[string]interface{}{"value": float64(util.Memory)}, time.Now())
			if err == nil {
				*out = append(*out, y)
			}
		}

		meminfo, ret := nvml.DeviceGetMemoryInfo(device)
		if ret == nvml.SUCCESS {
			t := float64(meminfo.Total) / (1024 * 1024)
			y, err := lp.New("mem_total", tags, map[string]interface{}{"value": t}, time.Now())
			if err == nil {
				*out = append(*out, y)
			}
			f := float64(meminfo.Used) / (1024 * 1024)
			y, err = lp.New("fb_memory", tags, map[string]interface{}{"value": f}, time.Now())
			if err == nil {
				*out = append(*out, y)
			}
		}

		temp, ret := nvml.DeviceGetTemperature(device, nvml.TEMPERATURE_GPU)
		if ret == nvml.SUCCESS {
			y, err := lp.New("temp", tags, map[string]interface{}{"value": float64(temp)}, time.Now())
			if err == nil {
				*out = append(*out, y)
			}
		}

		fan, ret := nvml.DeviceGetFanSpeed(device)
		if ret == nvml.SUCCESS {
			y, err := lp.New("fan", tags, map[string]interface{}{"value": float64(fan)}, time.Now())
			if err == nil {
				*out = append(*out, y)
			}
		}

		_, ecc_pend, ret := nvml.DeviceGetEccMode(device)
		if ret == nvml.SUCCESS {
			var y lp.MutableMetric
			var err error
			switch ecc_pend {
			case nvml.FEATURE_DISABLED:
				y, err = lp.New("ecc_mode", tags, map[string]interface{}{"value": string("OFF")}, time.Now())
			case nvml.FEATURE_ENABLED:
				y, err = lp.New("ecc_mode", tags, map[string]interface{}{"value": string("ON")}, time.Now())
			default:
				y, err = lp.New("ecc_mode", tags, map[string]interface{}{"value": string("UNKNOWN")}, time.Now())
			}
			if err == nil {
				*out = append(*out, y)
			}
		} else if ret == nvml.ERROR_NOT_SUPPORTED {
			y, err := lp.New("ecc_mode", tags, map[string]interface{}{"value": string("N/A")}, time.Now())
			if err == nil {
				*out = append(*out, y)
			}
		}

		pstate, ret := nvml.DeviceGetPerformanceState(device)
		if ret == nvml.SUCCESS {
			y, err := lp.New("perf_state", tags, map[string]interface{}{"value": fmt.Sprintf("P%d", int(pstate))}, time.Now())
			if err == nil {
				*out = append(*out, y)
			}
		}

		power, ret := nvml.DeviceGetPowerUsage(device)
		if ret == nvml.SUCCESS {
			y, err := lp.New("power_usage_report", tags, map[string]interface{}{"value": float64(power) / 1000}, time.Now())
			if err == nil {
				*out = append(*out, y)
			}
		}

		gclk, ret := nvml.DeviceGetClockInfo(device, nvml.CLOCK_GRAPHICS)
		if ret == nvml.SUCCESS {
			y, err := lp.New("graphics_clock_report", tags, map[string]interface{}{"value": float64(gclk)}, time.Now())
			if err == nil {
				*out = append(*out, y)
			}
		}

		smclk, ret := nvml.DeviceGetClockInfo(device, nvml.CLOCK_SM)
		if ret == nvml.SUCCESS {
			y, err := lp.New("sm_clock_report", tags, map[string]interface{}{"value": float64(smclk)}, time.Now())
			if err == nil {
				*out = append(*out, y)
			}
		}

		memclk, ret := nvml.DeviceGetClockInfo(device, nvml.CLOCK_MEM)
		if ret == nvml.SUCCESS {
			y, err := lp.New("mem_clock_report", tags, map[string]interface{}{"value": float64(memclk)}, time.Now())
			if err == nil {
				*out = append(*out, y)
			}
		}

		max_gclk, ret := nvml.DeviceGetMaxClockInfo(device, nvml.CLOCK_GRAPHICS)
		if ret == nvml.SUCCESS {
			y, err := lp.New("max_graphics_clock", tags, map[string]interface{}{"value": float64(max_gclk)}, time.Now())
			if err == nil {
				*out = append(*out, y)
			}
		}

		max_smclk, ret := nvml.DeviceGetClockInfo(device, nvml.CLOCK_SM)
		if ret == nvml.SUCCESS {
			y, err := lp.New("max_sm_clock", tags, map[string]interface{}{"value": float64(max_smclk)}, time.Now())
			if err == nil {
				*out = append(*out, y)
			}
		}

		max_memclk, ret := nvml.DeviceGetClockInfo(device, nvml.CLOCK_MEM)
		if ret == nvml.SUCCESS {
			y, err := lp.New("max_mem_clock", tags, map[string]interface{}{"value": float64(max_memclk)}, time.Now())
			if err == nil {
				*out = append(*out, y)
			}
		}

		ecc_db, ret := nvml.DeviceGetTotalEccErrors(device, 1, 1)
		if ret == nvml.SUCCESS {
			y, err := lp.New("ecc_db_error", tags, map[string]interface{}{"value": float64(ecc_db)}, time.Now())
			if err == nil {
				*out = append(*out, y)
			}
		}

		ecc_sb, ret := nvml.DeviceGetTotalEccErrors(device, 0, 1)
		if ret == nvml.SUCCESS {
			y, err := lp.New("ecc_sb_error", tags, map[string]interface{}{"value": float64(ecc_sb)}, time.Now())
			if err == nil {
				*out = append(*out, y)
			}
		}

		pwr_limit, ret := nvml.DeviceGetPowerManagementLimit(device)
		if ret == nvml.SUCCESS {
			y, err := lp.New("power_man_limit", tags, map[string]interface{}{"value": float64(pwr_limit)}, time.Now())
			if err == nil {
				*out = append(*out, y)
			}
		}

		enc_util, _, ret := nvml.DeviceGetEncoderUtilization(device)
		if ret == nvml.SUCCESS {
			y, err := lp.New("encoder_util", tags, map[string]interface{}{"value": float64(enc_util)}, time.Now())
			if err == nil {
				*out = append(*out, y)
			}
		}

		dec_util, _, ret := nvml.DeviceGetDecoderUtilization(device)
		if ret == nvml.SUCCESS {
			y, err := lp.New("decoder_util", tags, map[string]interface{}{"value": float64(dec_util)}, time.Now())
			if err == nil {
				*out = append(*out, y)
			}
		}
	}

}

func (m *NvidiaCollector) Close() {
	if m.init {
		nvml.Shutdown()
		m.init = false
	}
	return
}
