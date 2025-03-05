package collectors

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	lp "github.com/ClusterCockpit/cc-lib/ccMessage"
	cclog "github.com/ClusterCockpit/cc-metric-collector/pkg/ccLogger"
	"github.com/NVIDIA/go-nvml/pkg/nvml"
)

type NvidiaCollectorConfig struct {
	ExcludeMetrics        []string `json:"exclude_metrics,omitempty"`
	OnlyMetrics           []string `json:"only_metrics,omitempty"`
	ExcludeDevices        []string `json:"exclude_devices,omitempty"`
	AddPciInfoTag         bool     `json:"add_pci_info_tag,omitempty"`
	UsePciInfoAsTypeId    bool     `json:"use_pci_info_as_type_id,omitempty"`
	AddUuidMeta           bool     `json:"add_uuid_meta,omitempty"`
	AddBoardNumberMeta    bool     `json:"add_board_number_meta,omitempty"`
	AddSerialMeta         bool     `json:"add_serial_meta,omitempty"`
	ProcessMigDevices     bool     `json:"process_mig_devices,omitempty"`
	UseUuidForMigDevices  bool     `json:"use_uuid_for_mig_device,omitempty"`
	UseSliceForMigDevices bool     `json:"use_slice_for_mig_device,omitempty"`
	UseMemoryInfoV2       bool     `json:"use_memory_info_v2,omitempty"`
	SendDiffValues        bool     `json:"send_diff_values,omitempty"`
}

type NvidiaCollectorDevice struct {
	device         nvml.Device
	excludeMetrics map[string]bool
	tags           map[string]string
	meta           map[string]string
	config         NvidiaCollectorConfig
}

type NvidiaCollector struct {
	metricCollector
	config             NvidiaCollectorConfig
	gpus               []NvidiaCollectorDevice
	num_gpus           int
	prevEccStats       map[string]*eccStats
	prevRemappedStats  map[string]*remappedRowsStats
	prevNVLinkStats    map[string]*nvlinkStats
	prevViolationStats map[string]*violationStats
}

func (m *NvidiaCollector) CatchPanic() {
	if rerr := recover(); rerr != nil {
		log.Print(rerr)
		m.init = false
	}
}

// shouldOutput checks if a metric should be output based on onlyMetrics and excludeMetrics.
func (d *NvidiaCollectorDevice) shouldOutput(metric string) bool {
	if len(d.config.OnlyMetrics) > 0 {
		for _, m := range d.config.OnlyMetrics {
			if m == metric {
				return true
			}
		}
		return false
	}
	return !d.excludeMetrics[metric]
}

type eccStats struct {
	uncorrected uint64
	corrected   uint64
}

type remappedRowsStats struct {
	corrected   int
	uncorrected int
	pending     int
	failure     int
}

type violationStats struct {
	power          float64
	thermal        float64
	syncBoost      float64
	boardLimit     float64
	lowUtil        float64
	reliability    float64
	belowAppClock  float64
	belowBaseClock float64
}

type nvlinkStats struct {
	crcErrors      [nvml.NVLINK_MAX_LINKS]uint64 // Pro NVLink
	eccErrors      [nvml.NVLINK_MAX_LINKS]uint64
	replayErrors   [nvml.NVLINK_MAX_LINKS]uint64
	recoveryErrors [nvml.NVLINK_MAX_LINKS]uint64
	crcFlitErrors  [nvml.NVLINK_MAX_LINKS]uint64
	// Aggregierte Werte für _sum_diff
	aggregateCrcErrors      uint64
	aggregateEccErrors      uint64
	aggregateReplayErrors   uint64
	aggregateRecoveryErrors uint64
	aggregateCrcFlitErrors  uint64
}

func (m *NvidiaCollector) Init(config json.RawMessage) error {
	var err error
	m.name = "NvidiaCollector"
	m.config.AddPciInfoTag = false
	m.config.UsePciInfoAsTypeId = false
	m.config.ProcessMigDevices = false
	m.config.UseUuidForMigDevices = false
	m.config.UseSliceForMigDevices = false
	m.prevEccStats = make(map[string]*eccStats)
	m.prevRemappedStats = make(map[string]*remappedRowsStats)
	m.prevViolationStats = make(map[string]*violationStats)
	m.prevNVLinkStats = make(map[string]*nvlinkStats)
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
	for i := 0; i < num_gpus; i++ {

		// Skip excluded devices by ID
		str_i := fmt.Sprintf("%d", i)
		if _, skip := stringArrayContains(m.config.ExcludeDevices, str_i); skip {
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
		if _, skip := stringArrayContains(m.config.ExcludeDevices, pci_id); skip {
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

		// Add device config
		g.config = m.config

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
				cclog.ComponentError(m.name, "Unable to get board part number for device at index", i, ":", err.Error())
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

func sendMetric(metricName string, value interface{}, unit string, device NvidiaCollectorDevice, output chan lp.CCMessage, extraTags ...map[string]string) {
	msg, err := lp.NewMessage(metricName, device.tags, device.meta, map[string]interface{}{"value": value}, time.Now())
	if err != nil {
		return
	}
	if unit != "" {
		msg.AddMeta("unit", unit)
	}
	for _, tags := range extraTags {
		for k, v := range tags {
			msg.AddTag(k, v)
		}
	}
	output <- msg
}

func readMemoryInfo(device NvidiaCollectorDevice, config NvidiaCollectorConfig, output chan lp.CCMessage) error {
	// Try to use MemoryInfo_v2 if configured
	if config.UseMemoryInfoV2 {
		meminfoV2, ret := nvml.DeviceGetMemoryInfo_v2(device.device)
		if ret == nvml.SUCCESS {
			if device.shouldOutput("nv_fb_mem_total") {
				sendMetric("nv_fb_mem_total", float64(meminfoV2.Total)/(1024*1024), "MByte", device, output)
			}
			if device.shouldOutput("nv_fb_mem_used") {
				sendMetric("nv_fb_mem_used", float64(meminfoV2.Used)/(1024*1024), "MByte", device, output)
			}
			if device.shouldOutput("nv_fb_mem_reserved") {
				sendMetric("nv_fb_mem_reserved", float64(meminfoV2.Reserved)/(1024*1024), "MByte", device, output)
			}
			return nil
		}
	}

	// Fallback: Use DeviceGetMemoryInfo (v1)
	meminfo, ret := nvml.DeviceGetMemoryInfo(device.device)
	if ret != nvml.SUCCESS {
		return errors.New(nvml.ErrorString(ret))
	}
	if device.shouldOutput("nv_fb_mem_total") {
		sendMetric("nv_fb_mem_total", float64(meminfo.Total)/(1024*1024), "MByte", device, output)
	}
	if device.shouldOutput("nv_fb_mem_used") {
		sendMetric("nv_fb_mem_used", float64(meminfo.Used)/(1024*1024), "MByte", device, output)
	}
	return nil
}

func readBarMemoryInfo(device NvidiaCollectorDevice, config NvidiaCollectorConfig, output chan lp.CCMessage) error {
	meminfo, ret := nvml.DeviceGetBAR1MemoryInfo(device.device)
	if ret != nvml.SUCCESS {
		return errors.New(nvml.ErrorString(ret))
	}
	if device.shouldOutput("nv_bar1_mem_total") {
		sendMetric("nv_bar1_mem_total", float64(meminfo.Bar1Total)/(1024*1024), "MByte", device, output)
	}
	if device.shouldOutput("nv_bar1_mem_used") {
		sendMetric("nv_bar1_mem_used", float64(meminfo.Bar1Used)/(1024*1024), "MByte", device, output)
	}
	return nil
}

func readUtilization(device NvidiaCollectorDevice, config NvidiaCollectorConfig, output chan lp.CCMessage) error {
	isMig, ret := nvml.DeviceIsMigDeviceHandle(device.device)
	if ret != nvml.SUCCESS {
		return errors.New(nvml.ErrorString(ret))
	}
	if isMig {
		return nil
	}
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
		if device.shouldOutput("nv_util") {
			sendMetric("nv_util", float64(util.Gpu), "%", device, output)
		}
		if device.shouldOutput("nv_mem_util") {
			sendMetric("nv_mem_util", float64(util.Memory), "%", device, output)
		}
	}
	return nil
}

func readTemp(device NvidiaCollectorDevice, config NvidiaCollectorConfig, output chan lp.CCMessage) error {
	if device.shouldOutput("nv_temp") {
		// Retrieves the current temperature readings for the device, in degrees C.
		//
		// Available temperature sensors:
		// * TEMPERATURE_GPU: Temperature sensor for the GPU die.
		// * NVML_TEMPERATURE_COUNT
		temp, ret := nvml.DeviceGetTemperature(device.device, nvml.TEMPERATURE_GPU)
		if ret == nvml.SUCCESS {
			sendMetric("nv_temp", float64(temp), "degC", device, output)
		}
	}
	return nil
}

func readFan(device NvidiaCollectorDevice, config NvidiaCollectorConfig, output chan lp.CCMessage) error {
	if !device.shouldOutput("nv_fan") {
		return nil
	}
	// Retrieves the intended operating speed of the device's fan.
	//
	// Note: The reported speed is the intended fan speed.
	// If the fan is physically blocked and unable to spin, the output will not match the actual fan speed.
	//
	// For all discrete products with dedicated fans.
	//
	// The fan speed is expressed as a percentage of the product's maximum noise tolerance fan speed.
	// This value may exceed 100% in certain cases.
	//
	// If more than one fan is found we need to use DeviceGetFanSpeed_v2
	numFans, ret := nvml.DeviceGetNumFans(device.device)
	if ret != nvml.SUCCESS {
		return fmt.Errorf("Error retrieving number of fans: %v", ret)
	}

	if numFans <= 1 {
		fan, ret := nvml.DeviceGetFanSpeed(device.device)
		if ret == nvml.SUCCESS {
			sendMetric("nv_fan", float64(fan), "%", device, output)
		}
	} else {
		for i := 0; i < numFans; i++ {
			fan, ret := nvml.DeviceGetFanSpeed_v2(device.device, i)
			if ret == nvml.SUCCESS {
				sendMetric("nv_fan", float64(fan), "%", device, output, map[string]string{
					"stype":    "fan",
					"stype-id": fmt.Sprintf("%d", i),
				})
			}
		}
	}
	return nil
}

func readEccMode(device NvidiaCollectorDevice, config NvidiaCollectorConfig, output chan lp.CCMessage) error {
	if device.shouldOutput("nv_ecc_mode") {
		// Retrieves the current and pending ECC modes for the device.
		//
		// For Fermi or newer fully supported devices. Only applicable to devices with ECC.
		// Requires NVML_INFOROM_ECC version 1.0 or higher.
		//
		// Changing ECC modes requires a reboot.
		// The "pending" ECC mode refers to the target mode following the next reboot.
		_, eccPend, ret := nvml.DeviceGetEccMode(device.device)
		if ret == nvml.SUCCESS {
			var value string
			switch eccPend {
			case nvml.FEATURE_DISABLED:
				value = "OFF"
			case nvml.FEATURE_ENABLED:
				value = "ON"
			default:
				value = "UNKNOWN"
			}
			sendMetric("nv_ecc_mode", value, "", device, output)
		} else if ret == nvml.ERROR_NOT_SUPPORTED {
			sendMetric("nv_ecc_mode", "N/A", "", device, output)
		}
	}
	return nil
}

func readPerfState(device NvidiaCollectorDevice, config NvidiaCollectorConfig, output chan lp.CCMessage) error {
	if device.shouldOutput("nv_perf_state") {
		// Retrieves the current performance state for the device.
		//
		// Allowed PStates:
		//  0: Maximum Performance.
		// ..
		// 15: Minimum Performance.
		// 32: Unknown performance state.
		pState, ret := nvml.DeviceGetPerformanceState(device.device)
		if ret == nvml.SUCCESS {
			sendMetric("nv_perf_state", fmt.Sprintf("P%d", int(pState)), "", device, output)
		}
	}
	return nil
}

func readPowerUsage(device NvidiaCollectorDevice, config NvidiaCollectorConfig, output chan lp.CCMessage) error {
	if device.shouldOutput("nv_power_usage") {
		// Retrieves power usage for this GPU in milliwatts and its associated circuitry (e.g. memory)
		//
		// On Fermi and Kepler GPUs the reading is accurate to within +/- 5% of current power draw.
		//
		// It is only available if power management mode is supported
		mode, ret := nvml.DeviceGetPowerManagementMode(device.device)
		if ret != nvml.SUCCESS {
			return nil
		}
		if mode == nvml.FEATURE_ENABLED {
			power, ret := nvml.DeviceGetPowerUsage(device.device)
			if ret == nvml.SUCCESS {
				sendMetric("nv_power_usage", float64(power)/1000, "watts", device, output)
			}
		}
	}
	return nil
}

func readClocks(device NvidiaCollectorDevice, config NvidiaCollectorConfig, output chan lp.CCMessage) error {
	clockTypes := []struct {
		metricName string
		clockType  nvml.ClockType
		unit       string
	}{
		{"nv_graphics_clock", nvml.CLOCK_GRAPHICS, "MHz"},
		{"nv_sm_clock", nvml.CLOCK_SM, "MHz"},
		{"nv_mem_clock", nvml.CLOCK_MEM, "MHz"},
		{"nv_video_clock", nvml.CLOCK_VIDEO, "MHz"},
	}
	// Retrieves the current clock speeds for the device.
	//
	// Available clock information:
	// * CLOCK_GRAPHICS: Graphics clock domain.
	// * CLOCK_SM: Streaming Multiprocessor clock domain.
	// * CLOCK_MEM: Memory clock domain.
	for _, ct := range clockTypes {
		if device.shouldOutput(ct.metricName) {
			clock, ret := nvml.DeviceGetClockInfo(device.device, ct.clockType)
			if ret == nvml.SUCCESS {
				sendMetric(ct.metricName, float64(clock), ct.unit, device, output)
			}
		}
	}
	return nil
}

func readMaxClocks(device NvidiaCollectorDevice, config NvidiaCollectorConfig, output chan lp.CCMessage) error {
	clockTypes := []struct {
		metricName string
		clockType  nvml.ClockType
		unit       string
	}{
		{"nv_max_graphics_clock", nvml.CLOCK_GRAPHICS, "MHz"},
		{"nv_max_sm_clock", nvml.CLOCK_SM, "MHz"},
		{"nv_max_mem_clock", nvml.CLOCK_MEM, "MHz"},
		{"nv_max_video_clock", nvml.CLOCK_VIDEO, "MHz"},
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
	// On GPUs from Fermi family, current P0 clocks (reported by nvmlDeviceGetClockInfo) can differ from max clocks by a few MHz.
	for _, ct := range clockTypes {
		if device.shouldOutput(ct.metricName) {
			clock, ret := nvml.DeviceGetMaxClockInfo(device.device, ct.clockType)
			if ret == nvml.SUCCESS {
				sendMetric(ct.metricName, float64(clock), ct.unit, device, output)
			}
		}
	}
	return nil
}

func readEccErrors(device NvidiaCollectorDevice, config NvidiaCollectorConfig, output chan lp.CCMessage, prevStats *eccStats, deviceID string) error {
	var currentUncorrected, currentCorrected uint64
	var ret nvml.Return
	// Retrieves the total ECC error counts for the device.
	//
	// For Fermi or newer fully supported devices.
	// Only applicable to devices with ECC.
	// Requires NVML_INFOROM_ECC version 1.0 or higher.
	// Requires ECC Mode to be enabled.
	//
	// The total error count is the sum of errors across each of the separate memory systems,
	// i.e. the total set of errors across the entire device.
	if device.shouldOutput("nv_ecc_uncorrected_error") {
		currentUncorrected, ret = nvml.DeviceGetTotalEccErrors(device.device, nvml.MEMORY_ERROR_TYPE_UNCORRECTED, nvml.AGGREGATE_ECC)
		if ret == nvml.SUCCESS {
			sendMetric("nv_ecc_uncorrected_error", uint64(currentUncorrected), "", device, output)
		}
	}
	if device.shouldOutput("nv_ecc_corrected_error") {
		currentCorrected, ret = nvml.DeviceGetTotalEccErrors(device.device, nvml.MEMORY_ERROR_TYPE_CORRECTED, nvml.AGGREGATE_ECC)
		if ret == nvml.SUCCESS {
			sendMetric("nv_ecc_corrected_error", uint64(currentCorrected), "", device, output)
		}
	}
	if config.SendDiffValues {
		var diffUncorrected, diffCorrected uint64
		if prevStats.uncorrected == 0 && prevStats.corrected == 0 {
			diffUncorrected = 0
			diffCorrected = 0
		} else {
			diffUncorrected = currentUncorrected - prevStats.uncorrected
			diffCorrected = currentCorrected - prevStats.corrected
			if diffUncorrected > currentUncorrected {
				diffUncorrected = 0
			}
			if diffCorrected > currentCorrected {
				diffCorrected = 0
			}
		}
		prevStats.uncorrected = currentUncorrected
		prevStats.corrected = currentCorrected
		if device.shouldOutput("nv_ecc_uncorrected_error_diff") {
			sendMetric("nv_ecc_uncorrected_error_diff", uint64(diffUncorrected), "", device, output)
		}
		if device.shouldOutput("nv_ecc_corrected_error_diff") {
			sendMetric("nv_ecc_corrected_error_diff", uint64(diffCorrected), "", device, output)
		}
	}
	return nil
}

func readPowerLimit(device NvidiaCollectorDevice, config NvidiaCollectorConfig, output chan lp.CCMessage) error {
	if device.shouldOutput("nv_power_max_limit") {
		// Retrieves the power management limit associated with this device.
		//
		// For Fermi or newer fully supported devices.
		//
		// The power limit defines the upper boundary for the card's power draw.
		// If the card's total power draw reaches this limit the power management algorithm kicks in.
		pwrLimit, ret := nvml.DeviceGetPowerManagementLimit(device.device)
		if ret == nvml.SUCCESS {
			sendMetric("nv_power_max_limit", float64(pwrLimit)/1000, "watts", device, output)
		}
	}
	return nil
}

func readEncUtilization(device NvidiaCollectorDevice, config NvidiaCollectorConfig, output chan lp.CCMessage) error {
	isMig, ret := nvml.DeviceIsMigDeviceHandle(device.device)
	if ret != nvml.SUCCESS {
		return errors.New(nvml.ErrorString(ret))
	}
	if isMig {
		return nil
	}
	if device.shouldOutput("nv_encoder_util") {
		// Retrieves the current utilization and sampling size in microseconds for the Encoder
		//
		// For Kepler or newer fully supported devices.
		//
		// Note: On MIG-enabled GPUs, querying encoder utilization is not currently supported.
		encUtil, _, ret := nvml.DeviceGetEncoderUtilization(device.device)
		if ret == nvml.SUCCESS {
			sendMetric("nv_encoder_util", float64(encUtil), "%", device, output)
		}
	}
	return nil
}

func readDecUtilization(device NvidiaCollectorDevice, config NvidiaCollectorConfig, output chan lp.CCMessage) error {
	isMig, ret := nvml.DeviceIsMigDeviceHandle(device.device)
	if ret != nvml.SUCCESS {
		return errors.New(nvml.ErrorString(ret))
	}
	if isMig {
		return nil
	}
	if device.shouldOutput("nv_decoder_util") {
		// Retrieves the current utilization and sampling size in microseconds for the Decoder
		//
		// For Kepler or newer fully supported devices.
		//
		// Note: On MIG-enabled GPUs, querying encoder utilization is not currently supported.
		decUtil, _, ret := nvml.DeviceGetDecoderUtilization(device.device)
		if ret == nvml.SUCCESS {
			sendMetric("nv_decoder_util", float64(decUtil), "%", device, output)
		}
	}
	return nil
}

func readRemappedRows(device NvidiaCollectorDevice, config NvidiaCollectorConfig, output chan lp.CCMessage, prevStats *remappedRowsStats, deviceID string) error {

	// Get number of remapped rows. The number of rows reported will be based on the cause of the remapping.
	// isPending indicates whether or not there are pending remappings.
	// A reset will be required to actually remap the row.
	// failureOccurred will be set if a row remapping ever failed in the past.
	// A pending remapping won't affect future work on the GPU since error-containment and dynamic page blacklisting will take care of that.
	//
	// For Ampere or newer fully supported devices.
	//
	// Note: On MIG-enabled GPUs with active instances, querying the number of remapped rows is not supported
	corrected, uncorrected, pendingBool, failureBool, ret := nvml.DeviceGetRemappedRows(device.device)
	if ret != nvml.SUCCESS {
		return nil
	}
	var pending, failure int
	if pendingBool {
		pending = 1
	}
	if failureBool {
		failure = 1
	}
	if device.shouldOutput("nv_remapped_rows_corrected") {
		sendMetric("nv_remapped_rows_corrected", float64(corrected), "", device, output)
	}
	if device.shouldOutput("nv_remapped_rows_uncorrected") {
		sendMetric("nv_remapped_rows_uncorrected", float64(uncorrected), "", device, output)
	}
	if device.shouldOutput("nv_remapped_rows_pending") {
		sendMetric("nv_remapped_rows_pending", pending, "", device, output)
	}
	if device.shouldOutput("nv_remapped_rows_failure") {
		sendMetric("nv_remapped_rows_failure", failure, "", device, output)
	}
	if config.SendDiffValues {
		var diffCorrected, diffUncorrected, diffPending, diffFailure int
		if prevStats.corrected == 0 && prevStats.uncorrected == 0 && prevStats.pending == 0 && prevStats.failure == 0 {
			diffCorrected = 0
			diffUncorrected = 0
			diffPending = 0
			diffFailure = 0
		} else {
			diffCorrected = corrected - prevStats.corrected
			diffUncorrected = uncorrected - prevStats.uncorrected
			diffPending = pending - prevStats.pending
			diffFailure = failure - prevStats.failure
			if diffCorrected > corrected {
				diffCorrected = 0
			}
			if diffUncorrected > uncorrected {
				diffUncorrected = 0
			}
		}
		prevStats.corrected = corrected
		prevStats.uncorrected = uncorrected
		prevStats.pending = pending
		prevStats.failure = failure
		if device.shouldOutput("nv_remapped_rows_corrected_diff") {
			sendMetric("nv_remapped_rows_corrected_diff", float64(diffCorrected), "", device, output)
		}
		if device.shouldOutput("nv_remapped_rows_uncorrected_diff") {
			sendMetric("nv_remapped_rows_uncorrected_diff", float64(diffUncorrected), "", device, output)
		}
		if device.shouldOutput("nv_remapped_rows_pending_diff") {
			sendMetric("nv_remapped_rows_pending_diff", diffPending, "", device, output)
		}
		if device.shouldOutput("nv_remapped_rows_failure_diff") {
			sendMetric("nv_remapped_rows_failure_diff", diffFailure, "", device, output)
		}
	}
	return nil
}

func readProcessCounts(device NvidiaCollectorDevice, config NvidiaCollectorConfig, output chan lp.CCMessage) error {
	if device.shouldOutput("nv_compute_processes") {
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
			sendMetric("nv_compute_processes", len(procList), "", device, output)
		}
	}
	if device.shouldOutput("nv_graphics_processes") {
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
			sendMetric("nv_graphics_processes", len(procList), "", device, output)
		}
	}
	return nil
}

func readViolationStats(device NvidiaCollectorDevice, config NvidiaCollectorConfig, output chan lp.CCMessage, prevStats *violationStats) error {
	type violationMetric struct {
		name   string
		policy nvml.PerfPolicyType
	}

	// Gets the duration of time during which the device was throttled (lower than requested clocks) due to power
	//  or thermal constraints.
	//
	// The method is important to users who are tying to understand if their GPUs throttle at any point during their applications. The
	// difference in violation times at two different reference times gives the indication of GPU throttling event.
	//
	// Violation for thermal capping is not supported at this time.
	//
	// For Kepler  or newer fully supported devices.

	metrics := []violationMetric{
		{"nv_violation_power", nvml.PERF_POLICY_POWER},
		{"nv_violation_thermal", nvml.PERF_POLICY_THERMAL},
		{"nv_violation_sync_boost", nvml.PERF_POLICY_SYNC_BOOST},
		{"nv_violation_board_limit", nvml.PERF_POLICY_BOARD_LIMIT},
		{"nv_violation_low_util", nvml.PERF_POLICY_LOW_UTILIZATION},
		{"nv_violation_reliability", nvml.PERF_POLICY_RELIABILITY},
		{"nv_violation_below_app_clock", nvml.PERF_POLICY_TOTAL_APP_CLOCKS},
		{"nv_violation_below_base_clock", nvml.PERF_POLICY_TOTAL_BASE_CLOCKS},
	}
	for _, mtr := range metrics {
		if !device.shouldOutput(mtr.name) {
			continue
		}
		violTime, ret := nvml.DeviceGetViolationStatus(device.device, mtr.policy)
		if ret != nvml.SUCCESS {
			continue
		}
		currentValue := float64(violTime.ViolationTime) * 1e-9
		sendMetric(mtr.name, currentValue, "sec", device, output)
		if config.SendDiffValues && prevStats != nil {
			var diff float64
			var firstMeasurement bool
			switch mtr.name {
			case "nv_violation_power":
				firstMeasurement = prevStats.power == 0
			case "nv_violation_thermal":
				firstMeasurement = prevStats.thermal == 0
			case "nv_violation_sync_boost":
				firstMeasurement = prevStats.syncBoost == 0
			case "nv_violation_board_limit":
				firstMeasurement = prevStats.boardLimit == 0
			case "nv_violation_low_util":
				firstMeasurement = prevStats.lowUtil == 0
			case "nv_violation_reliability":
				firstMeasurement = prevStats.reliability == 0
			case "nv_violation_below_app_clock":
				firstMeasurement = prevStats.belowAppClock == 0
			case "nv_violation_below_base_clock":
				firstMeasurement = prevStats.belowBaseClock == 0
			}
			if firstMeasurement {
				diff = 0
			} else {
				var prevValue float64
				switch mtr.name {
				case "nv_violation_power":
					prevValue = prevStats.power
				case "nv_violation_thermal":
					prevValue = prevStats.thermal
				case "nv_violation_sync_boost":
					prevValue = prevStats.syncBoost
				case "nv_violation_board_limit":
					prevValue = prevStats.boardLimit
				case "nv_violation_low_util":
					prevValue = prevStats.lowUtil
				case "nv_violation_reliability":
					prevValue = prevStats.reliability
				case "nv_violation_below_app_clock":
					prevValue = prevStats.belowAppClock
				case "nv_violation_below_base_clock":
					prevValue = prevStats.belowBaseClock
				}
				diff = currentValue - prevValue
				if diff < 0 {
					diff = 0
				}
			}
			diffName := mtr.name + "_diff"
			if device.shouldOutput(diffName) {
				sendMetric(diffName, diff, "sec", device, output)
			}
			switch mtr.name {
			case "nv_violation_power":
				prevStats.power = currentValue
			case "nv_violation_thermal":
				prevStats.thermal = currentValue
			case "nv_violation_sync_boost":
				prevStats.syncBoost = currentValue
			case "nv_violation_board_limit":
				prevStats.boardLimit = currentValue
			case "nv_violation_low_util":
				prevStats.lowUtil = currentValue
			case "nv_violation_reliability":
				prevStats.reliability = currentValue
			case "nv_violation_below_app_clock":
				prevStats.belowAppClock = currentValue
			case "nv_violation_below_base_clock":
				prevStats.belowBaseClock = currentValue
			}
		}
	}
	return nil
}

func readNVLinkStats(device NvidiaCollectorDevice, config NvidiaCollectorConfig, output chan lp.CCMessage, prevStats *nvlinkStats, deviceID string) error {
	var aggregate_crc_errors uint64 = 0
	var aggregate_ecc_errors uint64 = 0
	var aggregate_replay_errors uint64 = 0
	var aggregate_recovery_errors uint64 = 0
	var aggregate_crc_flit_errors uint64 = 0

	// Retrieves the specified error counter value
	// Please refer to \a nvmlNvLinkErrorCounter_t for error counters that are available
	//
	// For Pascal &tm; or newer fully supported devices.

	needsMetric := func(base string) bool {
		return device.shouldOutput(base) ||
			device.shouldOutput(base+"_sum") ||
			(config.SendDiffValues && device.shouldOutput(base+"_diff")) ||
			(config.SendDiffValues && device.shouldOutput(base+"_sum_diff"))
	}

	for i := 0; i < nvml.NVLINK_MAX_LINKS; i++ {
		state, ret := nvml.DeviceGetNvLinkState(device.device, i)
		if ret != nvml.SUCCESS {
			continue
		}
		if state != nvml.FEATURE_ENABLED {
			continue
		}

		extraTags := map[string]string{
			"stype":    "nvlink",
			"stype-id": fmt.Sprintf("%d", i),
		}

		if needsMetric("nv_nvlink_crc_errors") {
			count, ret := nvml.DeviceGetNvLinkErrorCounter(device.device, i, nvml.NVLINK_ERROR_DL_CRC_DATA)
			if ret == nvml.SUCCESS {
				aggregate_crc_errors += count
				if device.shouldOutput("nv_nvlink_crc_errors") {
					sendMetric("nv_nvlink_crc_errors", count, "", device, output, extraTags)
				}
				if config.SendDiffValues && device.shouldOutput("nv_nvlink_crc_errors_diff") {
					var diff uint64
					if prevStats.crcErrors[i] == 0 {
						diff = 0
					} else {
						diff = count - prevStats.crcErrors[i]
						if diff > count {
							diff = 0
						}
					}
					sendMetric("nv_nvlink_crc_errors_diff", diff, "", device, output, extraTags)
					prevStats.crcErrors[i] = count
				}
			}
		}

		if needsMetric("nv_nvlink_ecc_errors") {
			count, ret := nvml.DeviceGetNvLinkErrorCounter(device.device, i, nvml.NVLINK_ERROR_DL_ECC_DATA)
			if ret == nvml.SUCCESS {
				aggregate_ecc_errors += count
				if device.shouldOutput("nv_nvlink_ecc_errors") {
					sendMetric("nv_nvlink_ecc_errors", count, "", device, output, extraTags)
				}
				if config.SendDiffValues && device.shouldOutput("nv_nvlink_ecc_errors_diff") {
					var diff uint64
					if prevStats.eccErrors[i] == 0 {
						diff = 0
					} else {
						diff = count - prevStats.eccErrors[i]
						if diff > count {
							diff = 0
						}
					}
					sendMetric("nv_nvlink_ecc_errors_diff", diff, "", device, output, extraTags)
					prevStats.eccErrors[i] = count
				}
			}
		}

		if needsMetric("nv_nvlink_replay_errors") {
			count, ret := nvml.DeviceGetNvLinkErrorCounter(device.device, i, nvml.NVLINK_ERROR_DL_REPLAY)
			if ret == nvml.SUCCESS {
				aggregate_replay_errors += count
				if device.shouldOutput("nv_nvlink_replay_errors") {
					sendMetric("nv_nvlink_replay_errors", count, "", device, output, extraTags)
				}
				if config.SendDiffValues && device.shouldOutput("nv_nvlink_replay_errors_diff") {
					var diff uint64
					if prevStats.replayErrors[i] == 0 {
						diff = 0
					} else {
						diff = count - prevStats.replayErrors[i]
						if diff > count {
							diff = 0
						}
					}
					sendMetric("nv_nvlink_replay_errors_diff", diff, "", device, output, extraTags)
					prevStats.replayErrors[i] = count
				}
			}
		}

		if needsMetric("nv_nvlink_recovery_errors") {
			count, ret := nvml.DeviceGetNvLinkErrorCounter(device.device, i, nvml.NVLINK_ERROR_DL_RECOVERY)
			if ret == nvml.SUCCESS {
				aggregate_recovery_errors += count
				if device.shouldOutput("nv_nvlink_recovery_errors") {
					sendMetric("nv_nvlink_recovery_errors", count, "", device, output, extraTags)
				}
				if config.SendDiffValues && device.shouldOutput("nv_nvlink_recovery_errors_diff") {
					var diff uint64
					if prevStats.recoveryErrors[i] == 0 {
						diff = 0
					} else {
						diff = count - prevStats.recoveryErrors[i]
						if diff > count {
							diff = 0
						}
					}
					sendMetric("nv_nvlink_recovery_errors_diff", diff, "", device, output, extraTags)
					prevStats.recoveryErrors[i] = count
				}
			}
		}

		if needsMetric("nv_nvlink_crc_flit_errors") {
			count, ret := nvml.DeviceGetNvLinkErrorCounter(device.device, i, nvml.NVLINK_ERROR_DL_CRC_FLIT)
			if ret == nvml.SUCCESS {
				aggregate_crc_flit_errors += count
				if device.shouldOutput("nv_nvlink_crc_flit_errors") {
					sendMetric("nv_nvlink_crc_flit_errors", count, "", device, output, extraTags)
				}
				if config.SendDiffValues && device.shouldOutput("nv_nvlink_crc_flit_errors_diff") {
					var diff uint64
					if prevStats.crcFlitErrors[i] == 0 {
						diff = 0
					} else {
						diff = count - prevStats.crcFlitErrors[i]
						if diff > count {
							diff = 0
						}
					}
					sendMetric("nv_nvlink_crc_flit_errors_diff", diff, "", device, output, extraTags)
					prevStats.crcFlitErrors[i] = count
				}
			}
		}
	}

	// Export aggregated values
	if device.shouldOutput("nv_nvlink_crc_errors_sum") {
		sendMetric("nv_nvlink_crc_errors_sum", aggregate_crc_errors, "", device, output, map[string]string{"stype": "nvlink"})
	}
	if device.shouldOutput("nv_nvlink_ecc_errors_sum") {
		sendMetric("nv_nvlink_ecc_errors_sum", aggregate_ecc_errors, "", device, output, map[string]string{"stype": "nvlink"})
	}
	if device.shouldOutput("nv_nvlink_replay_errors_sum") {
		sendMetric("nv_nvlink_replay_errors_sum", aggregate_replay_errors, "", device, output, map[string]string{"stype": "nvlink"})
	}
	if device.shouldOutput("nv_nvlink_recovery_errors_sum") {
		sendMetric("nv_nvlink_recovery_errors_sum", aggregate_recovery_errors, "", device, output, map[string]string{"stype": "nvlink"})
	}
	if device.shouldOutput("nv_nvlink_crc_flit_errors_sum") {
		sendMetric("nv_nvlink_crc_flit_errors_sum", aggregate_crc_flit_errors, "", device, output, map[string]string{"stype": "nvlink"})
	}

	// Export aggregated diff values
	if config.SendDiffValues {
		var diff_crc_sum, diff_ecc_sum, diff_replay_sum, diff_recovery_sum, diff_crc_flit_sum uint64

		// Initialize diffs to 0 for the first measurement
		if prevStats.aggregateCrcErrors == 0 && prevStats.aggregateEccErrors == 0 && prevStats.aggregateReplayErrors == 0 && prevStats.aggregateRecoveryErrors == 0 && prevStats.aggregateCrcFlitErrors == 0 {
			diff_crc_sum = 0
			diff_ecc_sum = 0
			diff_replay_sum = 0
			diff_recovery_sum = 0
			diff_crc_flit_sum = 0
		} else {
			// Compute diffs for sum metrics
			diff_crc_sum = aggregate_crc_errors - prevStats.aggregateCrcErrors
			diff_ecc_sum = aggregate_ecc_errors - prevStats.aggregateEccErrors
			diff_replay_sum = aggregate_replay_errors - prevStats.aggregateReplayErrors
			diff_recovery_sum = aggregate_recovery_errors - prevStats.aggregateRecoveryErrors
			diff_crc_flit_sum = aggregate_crc_flit_errors - prevStats.aggregateCrcFlitErrors

			// Reset diffs to 0 if they exceed current values (e.g., counter reset)
			if diff_crc_sum > aggregate_crc_errors {
				diff_crc_sum = 0
			}
			if diff_ecc_sum > aggregate_ecc_errors {
				diff_ecc_sum = 0
			}
			if diff_replay_sum > aggregate_replay_errors {
				diff_replay_sum = 0
			}
			if diff_recovery_sum > aggregate_recovery_errors {
				diff_recovery_sum = 0
			}
			if diff_crc_flit_sum > aggregate_crc_flit_errors {
				diff_crc_flit_sum = 0
			}
		}

		// Update prevStats with current aggregate values
		prevStats.aggregateCrcErrors = aggregate_crc_errors
		prevStats.aggregateEccErrors = aggregate_ecc_errors
		prevStats.aggregateReplayErrors = aggregate_replay_errors
		prevStats.aggregateRecoveryErrors = aggregate_recovery_errors
		prevStats.aggregateCrcFlitErrors = aggregate_crc_flit_errors

		// Export diff metrics for sum values
		if device.shouldOutput("nv_nvlink_crc_errors_sum_diff") {
			sendMetric("nv_nvlink_crc_errors_sum_diff", diff_crc_sum, "", device, output, map[string]string{"stype": "nvlink"})
		}
		if device.shouldOutput("nv_nvlink_ecc_errors_sum_diff") {
			sendMetric("nv_nvlink_ecc_errors_sum_diff", diff_ecc_sum, "", device, output, map[string]string{"stype": "nvlink"})
		}
		if device.shouldOutput("nv_nvlink_replay_errors_sum_diff") {
			sendMetric("nv_nvlink_replay_errors_sum_diff", diff_replay_sum, "", device, output, map[string]string{"stype": "nvlink"})
		}
		if device.shouldOutput("nv_nvlink_recovery_errors_sum_diff") {
			sendMetric("nv_nvlink_recovery_errors_sum_diff", diff_recovery_sum, "", device, output, map[string]string{"stype": "nvlink"})
		}
		if device.shouldOutput("nv_nvlink_crc_flit_errors_sum_diff") {
			sendMetric("nv_nvlink_crc_flit_errors_sum_diff", diff_crc_flit_sum, "", device, output, map[string]string{"stype": "nvlink"})
		}
	}

	return nil
}

func (m *NvidiaCollector) Read(interval time.Duration, output chan lp.CCMessage) {
	if !m.init {
		return
	}

	// Helper function to get the device name
	deviceName := func(device NvidiaCollectorDevice) string {
		name, ret := nvml.DeviceGetName(device.device)
		if ret != nvml.SUCCESS {
			return "NoName"
		}
		return name
	}

	// Helper function that executes a metric function and logs errors
	processMetric := func(metricName string, f func(NvidiaCollectorDevice, NvidiaCollectorConfig, chan lp.CCMessage) error, device NvidiaCollectorDevice) {
		if err := f(device, m.config, output); err != nil {
			cclog.ComponentDebug(m.name, fmt.Sprintf("%s for device %s failed", metricName, deviceName(device)))
		}
	}

	// Executes all metric functions for a device
	readAll := func(device NvidiaCollectorDevice) {
		processMetric("readMemoryInfo", readMemoryInfo, device)
		processMetric("readUtilization", readUtilization, device)
		processMetric("readTemp", readTemp, device)
		processMetric("readFan", readFan, device)
		processMetric("readEccMode", readEccMode, device)
		processMetric("readPerfState", readPerfState, device)
		processMetric("readPowerUsage", readPowerUsage, device)
		processMetric("readClocks", readClocks, device)
		processMetric("readMaxClocks", readMaxClocks, device)
		processMetric("readPowerLimit", readPowerLimit, device)
		processMetric("readEncUtilization", readEncUtilization, device)
		processMetric("readDecUtilization", readDecUtilization, device)
		processMetric("readBarMemoryInfo", readBarMemoryInfo, device)
		processMetric("readProcessCounts", readProcessCounts, device)
	}

	// Loop over all GPUs
	for i := 0; i < m.num_gpus; i++ {
		readAll(m.gpus[i])
		deviceID := m.gpus[i].tags["type-id"]

		if _, ok := m.prevEccStats[deviceID]; !ok {
			m.prevEccStats[deviceID] = &eccStats{}
		}
		readEccErrors(m.gpus[i], m.config, output, m.prevEccStats[deviceID], deviceID)

		if _, ok := m.prevRemappedStats[deviceID]; !ok {
			m.prevRemappedStats[deviceID] = &remappedRowsStats{}
		}
		readRemappedRows(m.gpus[i], m.config, output, m.prevRemappedStats[deviceID], deviceID)

		if _, ok := m.prevViolationStats[deviceID]; !ok {
			m.prevViolationStats[deviceID] = &violationStats{}
		}
		readViolationStats(m.gpus[i], m.config, output, m.prevViolationStats[deviceID])

		if _, ok := m.prevNVLinkStats[deviceID]; !ok {
			m.prevNVLinkStats[deviceID] = &nvlinkStats{}
		}
		readNVLinkStats(m.gpus[i], m.config, output, m.prevNVLinkStats[deviceID], deviceID)

		// If MIG devices should be processed
		if m.config.ProcessMigDevices {
			current, _, ret := nvml.DeviceGetMigMode(m.gpus[i].device)
			if ret != nvml.SUCCESS || current == nvml.DEVICE_MIG_DISABLE {
				continue
			}

			maxMig, ret := nvml.DeviceGetMaxMigDeviceCount(m.gpus[i].device)
			if ret != nvml.SUCCESS || maxMig == 0 {
				continue
			}
			cclog.ComponentDebug(m.name, "Reading MIG devices for GPU", i)

			for j := 0; j < maxMig; j++ {
				mdev, ret := nvml.DeviceGetMigDeviceHandleByIndex(m.gpus[i].device, j)
				if ret != nvml.SUCCESS {
					continue
				}

				excludeMetrics := make(map[string]bool)
				for _, metric := range m.config.ExcludeMetrics {
					excludeMetrics[metric] = true
				}

				// Initialize the MIG device and copy tags and meta data
				migDevice := NvidiaCollectorDevice{
					device:         mdev,
					tags:           make(map[string]string),
					meta:           make(map[string]string),
					excludeMetrics: excludeMetrics,
					config:         m.config,
				}
				for k, v := range m.gpus[i].tags {
					migDevice.tags[k] = v
				}
				migDevice.tags["stype"] = "mig"
				if m.config.UseUuidForMigDevices {
					uuid, ret := nvml.DeviceGetUUID(mdev)
					if ret != nvml.SUCCESS {
						cclog.ComponentError(m.name, "Unable to get UUID for mig device at index", j, ":", "error occurred")
					} else {
						migDevice.tags["stype-id"] = uuid
					}
				} else if m.config.UseSliceForMigDevices {
					name, ret := nvml.DeviceGetName(m.gpus[i].device)
					if ret == nvml.SUCCESS {
						mname, ret := nvml.DeviceGetName(mdev)
						if ret == nvml.SUCCESS {
							x := strings.Replace(mname, name, "", -1)
							x = strings.Replace(x, "MIG", "", -1)
							x = strings.TrimSpace(x)
							migDevice.tags["stype-id"] = x
						}
					}
				}
				if _, ok := migDevice.tags["stype-id"]; !ok {
					migDevice.tags["stype-id"] = fmt.Sprintf("%d", j)
				}
				for k, v := range m.gpus[i].meta {
					migDevice.meta[k] = v
				}
				if _, ok := migDevice.meta["uuid"]; ok && !m.config.UseUuidForMigDevices {
					uuid, ret := nvml.DeviceGetUUID(mdev)
					if ret == nvml.SUCCESS {
						migDevice.meta["uuid"] = uuid
					}
				}

				// Read all metrics for the MIG device
				readAll(migDevice)
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
