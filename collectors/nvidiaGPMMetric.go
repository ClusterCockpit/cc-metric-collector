package collectors

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	cclog "github.com/ClusterCockpit/cc-lib/v2/ccLogger"
	lp "github.com/ClusterCockpit/cc-lib/v2/ccMessage"
	"github.com/NVIDIA/go-nvml/pkg/nvml"
)

type NvidiaGPMMetricDef struct {
	name    string
	outname string
	id      nvml.GpmMetricId
	unit    string
}

var NvidiaGPMMetrics []NvidiaGPMMetricDef = []NvidiaGPMMetricDef{
	{
		name:    "GRAPHICS_UTIL",
		outname: "nv_gpm_graphics_util",
		id:      nvml.GPM_METRIC_GRAPHICS_UTIL,
		unit:    "%",
	},
	{
		name:    "SM_UTIL",
		outname: "nv_gpm_sm_util",
		id:      nvml.GPM_METRIC_SM_UTIL,
		unit:    "%",
	},
	{
		name:    "SM_OCCUPANCY",
		outname: "nv_gpm_sm_occupancy",
		id:      nvml.GPM_METRIC_SM_OCCUPANCY,
		unit:    "%",
	},
	{
		name:    "INTEGER_UTIL",
		outname: "nv_gpm_integer_util",
		id:      nvml.GPM_METRIC_INTEGER_UTIL,
		unit:    "%",
	},
	{
		name:    "ANY_TENSOR_UTIL",
		outname: "nv_gpm_any_tensor_util",
		id:      nvml.GPM_METRIC_ANY_TENSOR_UTIL,
		unit:    "%",
	},
	{
		name:    "DFMA_TENSOR_UTIL",
		outname: "nv_gpm_dfma_tensor_util",
		id:      nvml.GPM_METRIC_DFMA_TENSOR_UTIL,
		unit:    "%",
	},
	{
		name:    "HMMA_TENSOR_UTIL",
		outname: "nv_gpm_hmma_tensor_util",
		id:      nvml.GPM_METRIC_HMMA_TENSOR_UTIL,
		unit:    "%",
	},
	{
		name:    "IMMA_TENSOR_UTIL",
		outname: "nv_gpm_imma_tensor_util",
		id:      nvml.GPM_METRIC_IMMA_TENSOR_UTIL,
		unit:    "%",
	},
	{
		name:    "DRAM_BW_UTIL",
		outname: "nv_gpm_dram_bw_util",
		id:      nvml.GPM_METRIC_DRAM_BW_UTIL,
		unit:    "%",
	},
	{
		name:    "FP64_UTIL",
		outname: "nv_gpm_fp64_util",
		id:      nvml.GPM_METRIC_FP64_UTIL,
		unit:    "%",
	},
	{
		name:    "FP32_UTIL",
		outname: "nv_gpm_fp32_util",
		id:      nvml.GPM_METRIC_FP32_UTIL,
		unit:    "%",
	},
	{
		name:    "FP16_UTIL",
		outname: "nv_gpm_fp16_util",
		id:      nvml.GPM_METRIC_FP16_UTIL,
		unit:    "%",
	},
}

type NvidiaGPMCollectorConfig struct {
	Metrics               []string `json:"metrics,omitempty"`
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

type NvidiaGPMCollectorDevice struct {
	device        nvml.Device
	tags          map[string]string
	meta          map[string]string
	startTime     time.Time
	endTime       time.Time
	measurement   nvml.GpmMetricsGetType
	metricsLookup map[int]NvidiaGPMMetricDef
}

type NvidiaGPMCollector struct {
	metricCollector

	config   NvidiaGPMCollectorConfig
	gpus     []NvidiaGPMCollectorDevice
	num_gpus int
}

func (m *NvidiaGPMCollector) Init(config json.RawMessage) error {
	var err error = nil
	m.name = "NvidiaGPMCollector"
	m.parallel = true
	if err := m.setup(); err != nil {
		return fmt.Errorf("%s Init(): setup() call failed: %w", m.name, err)
	}
	if len(config) > 0 {
		d := json.NewDecoder(strings.NewReader(string(config)))
		d.DisallowUnknownFields()
		if err = d.Decode(&m.config); err != nil {
			return fmt.Errorf("%s Init(): Error decoding JSON config: %w", m.name, err)
		}
	}
	m.meta = map[string]string{
		"source": m.name,
		"group":  "NvidiaGPM",
	}

	// Initialize NVIDIA Management Library (NVML)
	ret := nvml.Init()

	// Error: NVML library not found
	// (nvml.ErrorString can not be used in this case)
	if ret == nvml.ERROR_LIBRARY_NOT_FOUND {
		return fmt.Errorf("%s Init(): NVML library not found", m.name)
	}
	if ret != nvml.SUCCESS {
		err = errors.New(nvml.ErrorString(ret))
		return fmt.Errorf("%s Init(): Unable to initialize NVML: %w", m.name, err)
	}

	// Number of NVIDIA GPUs
	num_gpus, ret := nvml.DeviceGetCount()
	if ret != nvml.SUCCESS {
		err = errors.New(nvml.ErrorString(ret))
		return fmt.Errorf("%s Init(): Unable to get device count: %w", m.name, err)
	}

	// For all GPUs
	m.gpus = make([]NvidiaGPMCollectorDevice, 0, num_gpus)
	for i := range num_gpus {

		// Skip excluded devices by ID
		str_i := strconv.Itoa(i)
		if slices.Contains(m.config.ExcludeDevices, str_i) {
			cclog.ComponentDebugf(m.name, "Skipping excluded device %s", str_i)
			continue
		}

		// Get device handle
		device, ret := nvml.DeviceGetHandleByIndex(i)
		if ret != nvml.SUCCESS {
			err = errors.New(nvml.ErrorString(ret))
			cclog.ComponentErrorf(m.name, "Unable to get device at index %d: %s", i, err.Error())
			continue
		}

		supportInfo, ret := nvml.GpmQueryDeviceSupport(device)
		if ret != nvml.SUCCESS {
			err = errors.New(nvml.ErrorString(ret))
			cclog.ComponentErrorf(m.name, "Unable to query GPM support for device at index %d: %s", i, err.Error())
			continue
		} else {
			if supportInfo.IsSupportedDevice == uint32(nvml.FEATURE_DISABLED) {
				cclog.ComponentErrorf(m.name, "Device at index %d does not support GPM metrics", i)
				continue
			}
		}

		stream, ret := nvml.GpmQueryIfStreamingEnabled(device)
		if ret != nvml.SUCCESS {
			err = errors.New(nvml.ErrorString(ret))
			cclog.ComponentErrorf(m.name, "Unable to query GPM streaming for device at index %d: %s", i, err.Error())
			continue
		} else {
			if stream == uint32(nvml.FEATURE_DISABLED) {
				ret = nvml.GpmSetStreamingEnabled(device, uint32(nvml.FEATURE_ENABLED))
				if ret != nvml.SUCCESS {
					err = errors.New(nvml.ErrorString(ret))
					cclog.ComponentErrorf(m.name, "Unable to set streaming mode for device at index %d: %s", i, err.Error())
				}
			}
		}

		// Get device's PCI info
		pciInfo, ret := nvml.DeviceGetPciInfo(device)
		if ret != nvml.SUCCESS {
			err = errors.New(nvml.ErrorString(ret))
			cclog.ComponentErrorf(m.name, "Unable to get PCI info for device at index %d: %s", i, err.Error())
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
			cclog.ComponentDebugf(m.name, "Skipping excluded device %s", pci_id)
			continue
		}
		ss, nvmlErr := nvml.GpmSampleAlloc()
		if nvmlErr != nvml.SUCCESS {
			err = errors.New(nvml.ErrorString(ret))
			cclog.ComponentErrorf(m.name, "Failed to allocate GPM sample for device %d: %s", i, err.Error())
			continue
		}
		es, nvmlErr := nvml.GpmSampleAlloc()
		if nvmlErr != nvml.SUCCESS {
			err = errors.New(nvml.ErrorString(ret))
			cclog.ComponentErrorf(m.name, "Failed to allocate GPM sample for device %d: %s", i, err.Error())
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
		g := NvidiaGPMCollectorDevice{}

		// Add device handle
		g.device = device

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
				err = errors.New(nvml.ErrorString(ret))
				cclog.ComponentError(m.name, "Unable to get boart part number for device at index", i, ":", err.Error())
			} else {
				g.meta["board_number"] = board
			}
		}
		if m.config.AddSerialMeta {
			serial, ret := nvml.DeviceGetSerial(device)
			if ret != nvml.SUCCESS {
				err = errors.New(nvml.ErrorString(ret))
				cclog.ComponentError(m.name, "Unable to get serial number for device at index", i, ":", err.Error())
			} else {
				g.meta["serial"] = serial
			}
		}
		if m.config.AddUuidMeta {
			uuid, ret := nvml.DeviceGetUUID(device)
			if ret != nvml.SUCCESS {
				err = errors.New(nvml.ErrorString(ret))
				cclog.ComponentError(m.name, "Unable to get UUID for device at index", i, ":", err.Error())
			} else {
				g.meta["uuid"] = uuid
			}
		}

		g.measurement.Sample1 = ss
		g.measurement.Sample2 = es
		g.measurement.Version = nvml.GPM_METRICS_GET_VERSION
		g.metricsLookup = make(map[int]NvidiaGPMMetricDef)
		metIdx := 0
		for _, inmetric := range m.config.Metrics {
			for _, defmetric := range NvidiaGPMMetrics {
				if inmetric == defmetric.outname || inmetric == defmetric.name {
					g.measurement.Metrics[metIdx] = nvml.GpmMetric{
						MetricId: uint32(defmetric.id),
					}
					g.metricsLookup[metIdx] = defmetric
					metIdx += 1
				}
			}
		}
		g.measurement.NumMetrics = uint32(metIdx)
		m.gpus = append(m.gpus, g)
	}
	cclog.ComponentDebugf(m.name, "Found %d Nvidia GPUs with GPM support", len(m.gpus))
	m.num_gpus = len(m.gpus)
	m.init = true
	return err
}

func (m *NvidiaGPMCollector) Read(interval time.Duration, output chan lp.CCMessage) {
	var err error
	if !m.init {
		return
	}
	for i, gpu := range m.gpus {
		gpu.startTime = time.Now()
		nvmlErr := gpu.measurement.Sample1.Get(gpu.device)
		if nvmlErr != nvml.SUCCESS {
			err = errors.New(nvml.ErrorString(nvmlErr))
			cclog.ComponentError(m.name, "Unable to get start GPM sample for device at index", i, ":", err.Error())
			continue
		}
	}
	time.Sleep(interval)

	for i, gpu := range m.gpus {
		gpu.endTime = time.Now()
		nvmlErr := gpu.measurement.Sample2.Get(gpu.device)
		if nvmlErr != nvml.SUCCESS {
			err = errors.New(nvml.ErrorString(nvmlErr))
			cclog.ComponentError(m.name, "Unable to get stop GPM sample for device at index", i, ":", err.Error())
			continue
		}
	}

	for i, gpu := range m.gpus {
		nvmlErr := nvml.GpmMetricsGet(&gpu.measurement)
		if nvmlErr != nvml.SUCCESS {
			err = errors.New(nvml.ErrorString(nvmlErr))
			cclog.ComponentError(m.name, "Unable to get evaluate GPM sample for device at index", i, ":", err.Error())
			continue
		}
		for idx, metricDef := range gpu.metricsLookup {
			y, err := lp.NewMetric(metricDef.outname, gpu.tags, gpu.meta, gpu.measurement.Metrics[idx].Value, time.Now())
			if err == nil {
				y.AddMeta("unit", metricDef.unit)
				output <- y
			}
		}
	}

}

func (m *NvidiaGPMCollector) Close() {
	if m.init {
		for i, gpu := range m.gpus {
			ret := gpu.measurement.Sample1.Free()
			if ret != nvml.SUCCESS {
				err := errors.New(nvml.ErrorString(ret))
				cclog.ComponentErrorf(m.name, "Unable to free start sample for device at index %d: %s", i, err.Error())
			}
			ret = gpu.measurement.Sample2.Free()
			if ret != nvml.SUCCESS {
				err := errors.New(nvml.ErrorString(ret))
				cclog.ComponentErrorf(m.name, "Unable to free stop sample for device at index %d: %s", i, err.Error())
			}
		}
		if ret := nvml.Shutdown(); ret != nvml.SUCCESS {
			cclog.ComponentError(m.name, "nvml.Shutdown() not successful")
		}
		m.init = false
	}
}
