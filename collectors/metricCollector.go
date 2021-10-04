package collectors

import (
	lp "github.com/influxdata/line-protocol"
	"io/ioutil"
	"log"
	"strconv"
	"strings"
	"time"
)

type MetricGetter interface {
	Name() string
	Init() error
	Read(time.Duration, *[]lp.MutableMetric)
	Close()
	//	GetNodeMetric() map[string]interface{}
	//	GetSocketMetrics() map[int]map[string]interface{}
	//	GetCpuMetrics() map[int]map[string]interface{}
}

type MetricCollector struct {
	name string
	init bool
	//	node    map[string]interface{}
	//	sockets map[int]map[string]interface{}
	//	cpus    map[int]map[string]interface{}
}

func (c *MetricCollector) Name() string {
	return c.name
}

//func (c *MetricCollector) GetNodeMetric() map[string]interface{} {
//	return c.node
//}

//func (c *MetricCollector) GetSocketMetrics() map[int]map[string]interface{} {
//	return c.sockets
//}

//func (c *MetricCollector) GetCpuMetrics() map[int]map[string]interface{} {
//	return c.cpus
//}

func (c *MetricCollector) setup() error {
	//	slist := SocketList()
	//	clist := CpuList()
	//	c.node = make(map[string]interface{})
	//	c.sockets = make(map[int]map[string]interface{}, len(slist))
	//	for _, s := range slist {
	//		c.sockets[s] = make(map[string]interface{})
	//	}
	//	c.cpus = make(map[int]map[string]interface{}, len(clist))
	//	for _, s := range clist {
	//		c.cpus[s] = make(map[string]interface{})
	//	}
	return nil
}

func intArrayContains(array []int, str int) (int, bool) {
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
