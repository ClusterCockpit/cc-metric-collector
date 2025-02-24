package sinks

import (
	"fmt"
	"strings"

	lp "github.com/ClusterCockpit/cc-lib/ccMessage"
)

func GangliaMetricName(point lp.CCMessage) string {
	name := point.Name()
	metricType, typeOK := point.GetTag("type")
	metricTid, tidOk := point.GetTag("type-id")
	gangliaType := metricType + metricTid
	if strings.Contains(name, metricType) && tidOk {
		name = strings.Replace(name, metricType, gangliaType, -1)
	} else if typeOK && tidOk {
		name = metricType + metricTid + "_" + name
	} else if point.HasTag("device") {
		device, _ := point.GetTag("device")
		name = name + "_" + device
	}

	return name
}

func GangliaMetricRename(name string) string {
	if name == "net_bytes_in" {
		return "bytes_in"
	} else if name == "net_bytes_out" {
		return "bytes_out"
	} else if name == "net_pkts_in" {
		return "pkts_in"
	} else if name == "net_pkts_out" {
		return "pkts_out"
	} else if name == "cpu_iowait" {
		return "cpu_wio"
	}
	return name
}

func GangliaSlopeType(point lp.CCMessage) uint {
	name := point.Name()
	if name == "mem_total" || name == "swap_total" {
		return 0
	}
	return 3
}

const DEFAULT_GANGLIA_METRIC_TMAX = 300
const DEFAULT_GANGLIA_METRIC_SLOPE = "both"

type GangliaMetric struct {
	Name  string
	Type  string
	Slope string
	Tmax  int
	Unit  string
}

type GangliaMetricGroup struct {
	Name    string
	Metrics []GangliaMetric
}

var CommonGangliaMetrics = []GangliaMetricGroup{
	{
		Name: "memory",
		Metrics: []GangliaMetric{
			{"mem_total", "float", "zero", 1200, "KB"},
			{"swap_total", "float", "zero", 1200, "KB"},
			{"mem_free", "float", "both", 180, "KB"},
			{"mem_shared", "float", "both", 180, "KB"},
			{"mem_buffers", "float", "both", 180, "KB"},
			{"mem_cached", "float", "both", 180, "KB"},
			{"swap_free", "float", "both", 180, "KB"},
			{"mem_sreclaimable", "float", "both", 180, "KB"},
			{"mem_slab", "float", "both", 180, "KB"},
		},
	},
	{
		Name: "cpu",
		Metrics: []GangliaMetric{
			{"cpu_num", "uint32", "zero", 1200, "CPUs"},
			{"cpu_speed", "uint32", "zero", 1200, "MHz"},
			{"cpu_user", "float", "both", 90, "%"},
			{"cpu_nice", "float", "both", 90, "%"},
			{"cpu_system", "float", "both", 90, "%"},
			{"cpu_idle", "float", "both", 3800, "%"},
			{"cpu_aidle", "float", "both", 90, "%"},
			{"cpu_wio", "float", "both", 90, "%"},
			{"cpu_intr", "float", "both", 90, "%"},
			{"cpu_sintr", "float", "both", 90, "%"},
			{"cpu_steal", "float", "both", 90, "%"},
			{"cpu_guest", "float", "both", 90, "%"},
			{"cpu_gnice", "float", "both", 90, "%"},
		},
	},
	{
		Name: "load",
		Metrics: []GangliaMetric{
			{"load_one", "float", "both", 70, ""},
			{"load_five", "float", "both", 325, ""},
			{"load_fifteen", "float", "both", 950, ""},
		},
	},
	{
		Name: "disk",
		Metrics: []GangliaMetric{
			{"disk_total", "double", "both", 1200, "GB"},
			{"disk_free", "double", "both", 180, "GB"},
			{"part_max_used", "float", "both", 180, "%"},
		},
	},
	{
		Name: "network",
		Metrics: []GangliaMetric{
			{"bytes_out", "float", "both", 300, "bytes/sec"},
			{"bytes_in", "float", "both", 300, "bytes/sec"},
			{"pkts_in", "float", "both", 300, "packets/sec"},
			{"pkts_out", "float", "both", 300, "packets/sec"},
		},
	},
	{
		Name: "process",
		Metrics: []GangliaMetric{
			{"proc_run", "uint32", "both", 950, ""},
			{"proc_total", "uint32", "both", 950, ""},
		},
	},
	{
		Name: "system",
		Metrics: []GangliaMetric{
			{"boottime", "uint32", "zero", 1200, "s"},
			{"sys_clock", "uint32", "zero", 1200, "s"},
			{"machine_type", "string", "zero", 1200, ""},
			{"os_name", "string", "zero", 1200, ""},
			{"os_release", "string", "zero", 1200, ""},
			{"mtu", "uint32", "both", 1200, ""},
		},
	},
}

type GangliaMetricConfig struct {
	Type  string
	Slope string
	Tmax  int
	Unit  string
	Group string
	Value string
	Name  string
}

func GetCommonGangliaConfig(point lp.CCMessage) GangliaMetricConfig {
	mname := GangliaMetricRename(point.Name())
	if oldname, ok := point.GetMeta("oldname"); ok {
		mname = GangliaMetricRename(oldname)
	}
	for _, group := range CommonGangliaMetrics {
		for _, metric := range group.Metrics {
			if metric.Name == mname {
				valueStr := ""
				value, ok := point.GetField("value")
				if ok {
					switch real := value.(type) {
					case float64:
						valueStr = fmt.Sprintf("%f", real)
					case float32:
						valueStr = fmt.Sprintf("%f", real)
					case int64:
						valueStr = fmt.Sprintf("%d", real)
					case int32:
						valueStr = fmt.Sprintf("%d", real)
					case int:
						valueStr = fmt.Sprintf("%d", real)
					case uint64:
						valueStr = fmt.Sprintf("%d", real)
					case uint32:
						valueStr = fmt.Sprintf("%d", real)
					case uint:
						valueStr = fmt.Sprintf("%d", real)
					case string:
						valueStr = real
					default:
					}
				}
				return GangliaMetricConfig{
					Group: group.Name,
					Type:  metric.Type,
					Slope: metric.Slope,
					Tmax:  metric.Tmax,
					Unit:  metric.Unit,
					Value: valueStr,
					Name:  GangliaMetricRename(mname),
				}
			}
		}
	}
	return GangliaMetricConfig{
		Group: "",
		Type:  "",
		Slope: "",
		Tmax:  0,
		Unit:  "",
		Value: "",
		Name:  "",
	}
}

func GetGangliaConfig(point lp.CCMessage) GangliaMetricConfig {
	mname := GangliaMetricRename(point.Name())
	if oldname, ok := point.GetMeta("oldname"); ok {
		mname = GangliaMetricRename(oldname)
	}
	group := ""
	if g, ok := point.GetMeta("group"); ok {
		group = g
	}
	unit := ""
	if u, ok := point.GetMeta("unit"); ok {
		unit = u
	}
	valueType := "double"
	valueStr := ""
	value, ok := point.GetField("value")
	if ok {
		switch real := value.(type) {
		case float64:
			valueStr = fmt.Sprintf("%f", real)
			valueType = "double"
		case float32:
			valueStr = fmt.Sprintf("%f", real)
			valueType = "float"
		case int64:
			valueStr = fmt.Sprintf("%d", real)
			valueType = "int32"
		case int32:
			valueStr = fmt.Sprintf("%d", real)
			valueType = "int32"
		case int:
			valueStr = fmt.Sprintf("%d", real)
			valueType = "int32"
		case uint64:
			valueStr = fmt.Sprintf("%d", real)
			valueType = "uint32"
		case uint32:
			valueStr = fmt.Sprintf("%d", real)
			valueType = "uint32"
		case uint:
			valueStr = fmt.Sprintf("%d", real)
			valueType = "uint32"
		case string:
			valueStr = real
			valueType = "string"
		default:
			valueType = "invalid"
		}
	}

	return GangliaMetricConfig{
		Group: group,
		Type:  valueType,
		Slope: DEFAULT_GANGLIA_METRIC_SLOPE,
		Tmax:  DEFAULT_GANGLIA_METRIC_TMAX,
		Unit:  unit,
		Value: valueStr,
		Name:  GangliaMetricRename(mname),
	}
}
