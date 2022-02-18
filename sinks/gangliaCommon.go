package sinks

import (
	"strings"

	lp "github.com/ClusterCockpit/cc-metric-collector/internal/ccMetric"
)

func GangliaMetricName(point lp.CCMetric) string {
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

func GangliaMetricRename(point lp.CCMetric) string {
	name := point.Name()
	if name == "mem_total" || name == "swap_total" {
		return name
	} else if name == "net_bytes_in" {
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

func GangliaSlopeType(point lp.CCMetric) uint {
	name := point.Name()
	if name == "mem_total" || name == "swap_total" {
		return 0
	}
	return 3
}
