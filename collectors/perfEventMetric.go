package collectors

/*
#cgo CFLAGS: -I/usr/include
#cgo LDFLAGS: -Wl,--unresolved-symbols=ignore-in-object-files
#include <stdlib.h>
#include <unistd.h>
#include <stdint.h>
#include <linux/perf_event.h>
#include <linux/hw_breakpoint.h>
#include <sys/ioctl.h>
#include <syscall.h>
#include <string.h>
#include <errno.h>

typedef enum {
	PERF_EVENT_WITH_CONFIG1 = (1<<0),
	PERF_EVENT_WITH_CONFIG2 = (1<<1),
	PERF_EVENT_WITH_EXCLUDE_KERNEL = (1<<2),
	PERF_EVENT_WITH_EXCLUDE_HV = (1<<3),
} PERF_EVENT_FLAG;

int perf_event_open(int type, uint64_t config, int cpu, uint64_t config1, uint64_t config2, int uncore)
{
	int ret;
	struct perf_event_attr attr;

	memset(&attr, 0, sizeof(struct perf_event_attr));
	attr.type = type;
	attr.config = config;
	if (!uncore) {
		attr.exclude_kernel = 1;
		attr.exclude_hv = 1;
	}
	//attr.disabled = 1;
	//
	// if (config1 > 0)
	// {
	// 	attr.config1 = config1;
	// }
	// if (config2 > 0)
	// {
	// 	attr.config2 = config2;
	// }
	// if (flags & PERF_EVENT_WITH_CONFIG1)
	// {
	// 	attr.config1 = config1;
	// }
	// if (flags & PERF_EVENT_WITH_CONFIG2)
	// {
	// 	attr.config2 = config2;
	// }
	// if (flags & PERF_EVENT_WITH_EXCLUDE_KERNEL)
	// {
	// 	attr.exclude_kernel = 1;
	// }
	// if (flags & PERF_EVENT_WITH_EXCLUDE_HV)
	// {
	// 	attr.exclude_hv = 1;
	// }



	ret = syscall(__NR_perf_event_open, &attr, -1, cpu, -1, 0);
	if (ret < 0)
	{
		return -errno;
	}
	return 0;
}

int perf_event_stop(int fd)
{
	return ioctl(fd, PERF_EVENT_IOC_DISABLE, 0);
}


int perf_event_start(int fd)
{
	return ioctl(fd, PERF_EVENT_IOC_ENABLE, 0);
}

int perf_event_reset(int fd)
{
	return ioctl(fd, PERF_EVENT_IOC_RESET, 0);
}

int perf_event_read(int fd, uint64_t *data)
{
	int ret = 0;

	ret = read(fd, data, sizeof(uint64_t));
	if (ret != sizeof(uint64_t))
	{
		return -errno;
	}
	return 0;
}

int perf_event_close(int fd)
{
	close(fd);
}

*/
import "C"

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	lp "github.com/ClusterCockpit/cc-energy-manager/pkg/cc-message"
	cclog "github.com/ClusterCockpit/cc-metric-collector/pkg/ccLogger"
	"github.com/ClusterCockpit/cc-metric-collector/pkg/ccTopology"
)

const SYSFS_PERF_EVENT_PATH = `/sys/devices`

type PerfEventCollectorEventConfig struct {
	Name              string `json:"name"`
	Unit              string `json:"unit,omitempty"`
	unitType          int
	Config            string `json:"config"`
	config            C.uint64_t
	Config1           string `json:"config1,omitempty"`
	config1           C.uint64_t
	Config2           string `json:"config2,omitempty"`
	config2           C.uint64_t
	ExcludeKernel     bool              `json:"exclude_kernel,omitempty"`
	ExcludeHypervisor bool              `json:"exclude_hypervisor,omitempty"`
	Tags              map[string]string `json:"tags,omitempty"`
	Meta              map[string]string `json:"meta,omitempty"`
	PerHwthread       bool              `json:"per_hwthread,omitempty"`
	PerSocket         bool              `json:"per_socket,omitempty"`
	ScaleFile         string            `json:"scale_file,omitempty"`
	scaling_factor    float64
	flags             uint64
	valid             bool
	cpumask           []int
}

type PerfEventCollectorEventData struct {
	fd        C.int
	last      uint64
	last_diff uint64
	idx       int
}

type PerfEventCollectorConfig struct {
	Events []PerfEventCollectorEventConfig `json:"events"`
	events []PerfEventCollectorEventConfig
}

type PerfEventCollector struct {
	metricCollector
	config PerfEventCollectorConfig // the configuration structure
	meta   map[string]string        // default meta information
	tags   map[string]string        // default tags
	events map[int]map[int]PerfEventCollectorEventData
}

func UpdateEventConfig(event *PerfEventCollectorEventConfig) error {
	parseHexNumber := func(number string) (uint64, error) {
		snum := strings.Trim(number, "\n")
		snum = strings.Replace(snum, "0x", "", -1)
		snum = strings.Replace(snum, "0X", "", -1)
		return strconv.ParseUint(snum, 16, 64)
	}
	if len(event.Unit) == 0 {
		event.Unit = "cpu"
	}

	unitpath := path.Join(SYSFS_PERF_EVENT_PATH, event.Unit)
	if _, err := os.Stat(unitpath); err != nil {
		return err
	}
	typefile := path.Join(unitpath, "type")
	if _, err := os.Stat(typefile); err != nil {
		return err
	}
	typebytes, err := os.ReadFile(typefile)
	if err != nil {
		return err
	}
	typestring := string(typebytes)
	ut, err := strconv.ParseUint(strings.Trim(typestring, "\n"), 10, 64)
	if err != nil {
		return err
	}
	event.unitType = int(ut)

	if len(event.Config) > 0 {
		x, err := parseHexNumber(event.Config)
		if err != nil {
			return err
		}
		event.config = C.uint64_t(x)
	}
	if len(event.Config1) > 0 {
		x, err := parseHexNumber(event.Config1)
		if err != nil {
			return err
		}
		event.config1 = C.uint64_t(x)
	}
	if len(event.Config2) > 0 {
		x, err := parseHexNumber(event.Config2)
		if err != nil {
			return err
		}
		event.config2 = C.uint64_t(x)
	}
	if len(event.ScaleFile) > 0 {
		if _, err := os.Stat(event.ScaleFile); err != nil {
			return err
		}
		scalebytes, err := os.ReadFile(event.ScaleFile)
		if err != nil {
			return err
		}
		x, err := strconv.ParseFloat(string(scalebytes), 64)
		if err != nil {
			return err
		}
		event.scaling_factor = x
	}
	event.cpumask = make([]int, 0)
	cpumaskfile := path.Join(unitpath, "cpumask")
	if _, err := os.Stat(cpumaskfile); err == nil {

		cpumaskbytes, err := os.ReadFile(cpumaskfile)
		if err != nil {
			return err
		}
		cpumaskstring := strings.Trim(string(cpumaskbytes), "\n")
		cclog.Debug("cpumask", cpumaskstring)
		for _, part := range strings.Split(cpumaskstring, ",") {
			start := 0
			end := 0
			count, _ := fmt.Sscanf(part, "%d-%d", &start, &end)
			cclog.Debug("scanf", count, " s ", start, " e ", end)

			if count == 1 {
				cclog.Debug("adding ", start)
				event.cpumask = append(event.cpumask, start)
			} else if count == 2 {
				for i := start; i <= end; i++ {
					cclog.Debug("adding ", i)
					event.cpumask = append(event.cpumask, i)
				}
			}

		}
	} else {
		event.cpumask = append(event.cpumask, ccTopology.CpuList()...)
	}

	event.valid = true
	return nil
}

func (m *PerfEventCollector) Init(config json.RawMessage) error {
	var err error = nil

	m.name = "PerfEventCollector"

	m.setup()

	m.parallel = false

	m.meta = map[string]string{"source": m.name, "group": "PerfCounter"}

	m.tags = map[string]string{"type": "node"}

	cpudata := ccTopology.CpuData()

	if len(config) > 0 {
		err = json.Unmarshal(config, &m.config)
		if err != nil {
			cclog.ComponentError(m.name, "Error reading config:", err.Error())
			return err
		}
	}

	for i, e := range m.config.Events {
		err = UpdateEventConfig(&e)
		if err != nil {
			cclog.ComponentError(m.name, "Checks for event unit", e.Name, "failed:", err.Error())
		}
		m.config.Events[i] = e
	}
	total := 0
	m.events = make(map[int]map[int]PerfEventCollectorEventData)
	for _, hwt := range cpudata {
		cclog.ComponentDebug(m.name, "Adding events for cpuid", hwt.CpuID)
		hwt_events := make(map[int]PerfEventCollectorEventData)
		for j, e := range m.config.Events {
			if e.valid {
				if _, ok := intArrayContains(e.cpumask, hwt.CpuID); ok {
					cclog.ComponentDebug(m.name, "Adding event", e.Name, fmt.Sprintf("(cpuid %d unit %s(%d) config %s config1 %s config2 %s)",
						hwt.CpuID,
						e.Unit,
						e.unitType,
						e.Config,
						e.Config1,
						e.Config2,
					))
					// (int type, uint64_t config, int cpu, uint64_t config1, uint64_t config2, int uncore)
					fd := C.perf_event_open(C.int(e.unitType), e.config, C.int(hwt.CpuID), e.config1, e.config2, C.int(1))
					if fd < 0 {
						cclog.ComponentError(m.name, "Failed to create event", e.Name, ":", fd)
						continue
					}
					hwt_events[j] = PerfEventCollectorEventData{
						idx:  j,
						fd:   fd,
						last: 0,
					}
					total++
				} else {
					cclog.ComponentDebug(m.name, "Cpu not in cpumask of unit", e.cpumask)
					hwt_events[j] = PerfEventCollectorEventData{
						idx:  j,
						fd:   -1,
						last: 0,
					}
				}
			} else {
				cclog.ComponentError(m.name, "Event", e.Name, "not valid")
			}
		}
		cclog.ComponentDebug(m.name, "Adding", len(hwt_events), "events for cpuid", hwt.CpuID)
		m.events[hwt.CpuID] = hwt_events
	}
	if total == 0 {
		cclog.ComponentError(m.name, "Failed to add events")
		return errors.New("failed to add events")
	}

	m.init = true
	return err
}

func (m *PerfEventCollector) CalcSocketData() map[int]map[int]interface{} {
	out := make(map[int]map[int]interface{})

	for cpuid, cpudata := range m.events {
		for i, eventdata := range cpudata {
			eventconfig := m.config.Events[i]
			sid := ccTopology.GetHwthreadSocket(cpuid)
			if _, ok := out[sid]; !ok {
				out[sid] = make(map[int]interface{})
				for i := range cpudata {
					out[sid][i] = 0
				}
			}
			if eventconfig.scaling_factor != 0 {
				out[sid][i] = out[sid][i].(float64) + (float64(eventdata.last_diff) * eventconfig.scaling_factor)
			} else {
				out[sid][i] = out[sid][i].(uint64) + eventdata.last_diff
			}
		}
	}

	return out
}

func (m *PerfEventCollector) Read(interval time.Duration, output chan lp.CCMessage) {

	timestamp := time.Now()

	var wg sync.WaitGroup

	for cpuid := range m.events {
		wg.Add(1)
		go func(cpuid int, data map[int]map[int]PerfEventCollectorEventData, wg *sync.WaitGroup) {
			var err error = nil
			var events map[int]PerfEventCollectorEventData = data[cpuid]
			for i, e := range events {

				var data C.uint64_t = 0
				if e.fd < 0 {
					continue
				}
				ret := C.perf_event_read(e.fd, &data)
				if ret < 0 {
					event := m.config.Events[i]
					cclog.ComponentError(m.name, "Failed to read event", event.Name, ":", ret)
				}
				if e.last == 0 {
					cclog.ComponentDebug(m.name, "Updating last value on first iteration")
					e.last = uint64(data)

				} else {
					var metric lp.CCMetric
					event := m.config.Events[i]
					value := uint64(data) - e.last
					cclog.ComponentDebug(m.name, "Calculating difference", uint64(data), "-", e.last, "=", uint64(data)-e.last)
					e.last = uint64(data)
					e.last_diff = value

					if event.scaling_factor == 0 {
						metric, err = lp.NewMetric(event.Name, m.tags, m.meta, value, timestamp)
					} else {
						var f64_value float64 = float64(value) * event.scaling_factor
						metric, err = lp.NewMetric(event.Name, m.tags, m.meta, f64_value, timestamp)
					}
					//if event.PerHwthread {
					if err == nil {
						metric.AddTag("type", "hwthread")
						metric.AddTag("type-id", fmt.Sprintf("%d", cpuid))
						for k, v := range event.Tags {
							metric.AddTag(k, v)
						}
						for k, v := range event.Meta {
							metric.AddMeta(k, v)
						}
						output <- metric
					} else {
						cclog.ComponentError(m.name, "Failed to create CCMetric for event", event.Name)
					}
					//}
				}
				events[i] = e
			}
			data[cpuid] = events
			wg.Done()
		}(cpuid, m.events, &wg)
	}
	wg.Wait()

	// 	var data C.uint64_t = 0
	// 	event := m.config.Events[e.idx]
	// 	cclog.ComponentDebug(m.name, "Reading event", event.Name)
	// 	ret := C.perf_event_read(e.fd, &data)
	// 	if ret < 0 {
	// 		cclog.ComponentError(m.name, "Failed to read event", event.Name, ":", ret)
	// 	}
	// 	if e.last == 0 {
	// 		cclog.ComponentDebug(m.name, "Updating last value on first iteration")
	// 		e.last = uint64(data)

	// 	} else {
	// 		value := uint64(data) - e.last
	// 		cclog.ComponentDebug(m.name, "Calculating difference", uint64(data), "-", e.last, "=", uint64(data)-e.last)
	// 		e.last = uint64(data)

	// 		y, err := lp.NewMetric(event.Name, m.tags, m.meta, value, timestamp)
	// 		if err == nil {
	// 			for k, v := range event.Tags {
	// 				y.AddTag(k, v)
	// 			}
	// 			for k, v := range event.Meta {
	// 				y.AddMeta(k, v)
	// 			}
	// 			output <- y
	// 		} else {
	// 			cclog.ComponentError(m.name, "Failed to create CCMetric for event", event.Name)
	// 		}
	// 	}
	// 	m.events[i] = e
	// }

}

func (m *PerfEventCollector) Close() {

	for _, events := range m.events {
		for _, e := range events {
			C.perf_event_close(e.fd)
		}
	}
	m.init = false
}
