package collectors

import (
	"encoding/json"
	"errors"
	lp "github.com/ClusterCockpit/cc-metric-collector/internal/ccMetric"
	influx "github.com/influxdata/line-protocol"
	"io/ioutil"
	"log"
	"strconv"
	"strings"
	"time"
)

type MetricCollector interface {
	Name() string
	Init(config json.RawMessage) error
	Initialized() bool
	Read(duration time.Duration, output chan lp.CCMetric)
	Close()
}

type metricCollector struct {
	output chan lp.CCMetric
	name   string
	init   bool
	meta   map[string]string
}

func (c *metricCollector) Name() string {
	return c.name
}

func (c *metricCollector) setup() error {
	return nil
}

func (c *metricCollector) Initialized() bool {
	return c.init == true
}

func intArrayContains(array []int, str int) (int, bool) {
	for i, a := range array {
		if a == str {
			return i, true
		}
	}
	return -1, false
}

func stringArrayContains(array []string, str string) (int, bool) {
	for i, a := range array {
		if a == str {
			return i, true
		}
	}
	return -1, false
}

func SocketList() []int {
	buffer, err := ioutil.ReadFile("/proc/cpuinfo")
	if err != nil {
		log.Print(err)
		return nil
	}
	ll := strings.Split(string(buffer), "\n")
	var packs []int
	for _, line := range ll {
		if strings.HasPrefix(line, "physical id") {
			lv := strings.Fields(line)
			id, err := strconv.ParseInt(lv[3], 10, 32)
			if err != nil {
				log.Print(err)
				return packs
			}
			_, found := intArrayContains(packs, int(id))
			if !found {
				packs = append(packs, int(id))
			}
		}
	}
	return packs
}

func CpuList() []int {
	buffer, err := ioutil.ReadFile("/proc/cpuinfo")
	if err != nil {
		log.Print(err)
		return nil
	}
	ll := strings.Split(string(buffer), "\n")
	var cpulist []int
	for _, line := range ll {
		if strings.HasPrefix(line, "processor") {
			lv := strings.Fields(line)
			id, err := strconv.ParseInt(lv[2], 10, 32)
			if err != nil {
				log.Print(err)
				return cpulist
			}
			_, found := intArrayContains(cpulist, int(id))
			if !found {
				cpulist = append(cpulist, int(id))
			}
		}
	}
	return cpulist
}

func Tags2Map(metric influx.Metric) map[string]string {
	tags := make(map[string]string)
	for _, t := range metric.TagList() {
		tags[t.Key] = t.Value
	}
	return tags
}

func Fields2Map(metric influx.Metric) map[string]interface{} {
	fields := make(map[string]interface{})
	for _, f := range metric.FieldList() {
		fields[f.Key] = f.Value
	}
	return fields
}

func RemoveFromStringList(s []string, r string) ([]string, error) {
	for i, item := range s {
		if r == item {
			return append(s[:i], s[i+1:]...), nil
		}
	}
	return s, errors.New("No such string in list")
}
