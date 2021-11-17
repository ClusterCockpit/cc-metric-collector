package collectors

import (
	"errors"
	"fmt"
	lp "github.com/influxdata/line-protocol"
	"io/ioutil"
	"log"
	"strconv"
	"strings"
	"time"
)

const IPMITOOL_PATH = `/usr/bin/ipmitool`
const IPMISENSORS_PATH = `/usr/sbin/ipmi-sensors`

type IpmiCollector struct {
	MetricCollector
	tags    map[string]string
	matches map[string]string
}

func (m *IpmiCollector) Init() error {
	m.name = "IpmiCollector"
	m.setup()
	m.init = true
}

func ReadIpmiTool(out *[]lp.MutableMetric) {
	command := exec.Command(string(IPMITOOL_PATH), "")
	command.Wait()
	stdout, err := command.Output()
	if err != nil {
		log.Print(err)
		return
	}

	ll := strings.Split(string(stdout), "\n")

	for _, line := range ll {
		lv := strings.Split(line, ",")
		v, err := strconv.ParseFloat(lv[3], 64)
		if err == nil {
			name := strings.ToLower(strings.Replace(lv[1], " ", "_", -1))
			y, err := lp.New(name, map[string]string{}, map[string]interface{}{"value": v}, time.Now())
			if err == nil {
				*out = append(*out, y)
			}
		}
	}
}

func ReadIpmiSensors(out *[]lp.MutableMetric) {

	command := exec.Command(string(IPMISENSORS_PATH), "--comma-separated-output")
	command.Wait()
	stdout, err := command.Output()
	if err != nil {
		log.Print(err)
		return
	}

	ll := strings.Split(string(stdout), "\n")

	for _, line := range ll {
		lv := strings.Split(line, ",")
		v, err := strconv.ParseFloat(lv[3], 64)
		if err == nil {
			name := strings.ToLower(strings.Replace(lv[1], " ", "_", -1))
			y, err := lp.New(name, map[string]string{"unit": lv[4]}, map[string]interface{}{"value": v}, time.Now())
			if err == nil {
				*out = append(*out, y)
			}
		}
	}
}

func (m *IpmiCollector) Read(interval time.Duration, out *[]lp.MutableMetric) {
	_, err := ioutil.ReadFile(string(IPMITOOL_PATH))
	if err == nil {
		ReadIpmiTool(out)
	} else {
		_, err := ioutil.ReadFile(string(IPMISENSORS_PATH))
		if err == nil {
			ReadIpmiSensors(out)
		}
	}
}

func (m *IpmiCollector) Close() {
	m.init = false
	return
}
