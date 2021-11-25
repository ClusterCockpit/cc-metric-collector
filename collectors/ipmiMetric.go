package collectors

import (
	"errors"
	lp "github.com/influxdata/line-protocol"
	"log"
	"strconv"
	"strings"
	"time"
	"os"
	"os/exec"
	"encoding/json"
)

const IPMITOOL_PATH = `/usr/bin/ipmitool`
const IPMISENSORS_PATH = `/usr/sbin/ipmi-sensors`

type IpmiCollectorConfig struct {
    ExcludeDevices []string `json:"exclude_devices"`
    IpmitoolPath string `json:"ipmitool_path"`
    IpmisensorsPath string `json:"ipmisensors_path"`
}

type IpmiCollector struct {
	MetricCollector
	tags    map[string]string
	matches map[string]string
	config IpmiCollectorConfig
}

func (m *IpmiCollector) Init(config []byte) error {
	m.name = "IpmiCollector"
	m.setup()
	if len(config) > 0 {
		err := json.Unmarshal(config, &m.config)
		if err != nil {
			return err
		}
	}
	_, err1 := os.Stat(m.config.IpmitoolPath)
	_, err2 := os.Stat(m.config.IpmisensorsPath)
	if err1 != nil && err2 != nil {
	    return errors.New("No IPMI reader found")
	}
	m.init = true
	return nil
}

func ReadIpmiTool(cmd string, out *[]lp.MutableMetric) {
	command := exec.Command(cmd, "")
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
			y, err := lp.New(name, map[string]string{"type" : "node"}, map[string]interface{}{"value": v}, time.Now())
			if err == nil {
				*out = append(*out, y)
			}
		}
	}
}

func ReadIpmiSensors(cmd string, out *[]lp.MutableMetric) {

	command := exec.Command(cmd, "--comma-separated-output")
	command.Wait()
	stdout, err := command.Output()
	if err != nil {
		log.Print(err)
		return
	}

	ll := strings.Split(string(stdout), "\n")

	for _, line := range ll {
		lv := strings.Split(line, ",")
		if len(lv) > 3 {
		    v, err := strconv.ParseFloat(lv[3], 64)
		    if err == nil {
			    name := strings.ToLower(strings.Replace(lv[1], " ", "_", -1))
			    y, err := lp.New(name, map[string]string{"unit": lv[4], "type" : "node"}, map[string]interface{}{"value": v}, time.Now())
			    if err == nil {
				    *out = append(*out, y)
			    }
		    }
		}
	}
}

func (m *IpmiCollector) Read(interval time.Duration, out *[]lp.MutableMetric) {
    if len(m.config.IpmitoolPath) > 0 {
	    _, err := os.Stat(m.config.IpmitoolPath)
	    if err == nil {
		    ReadIpmiTool(m.config.IpmitoolPath, out)
		}
	} else if len(m.config.IpmisensorsPath) > 0 {
	    _, err := os.Stat(m.config.IpmisensorsPath)
		if err == nil {
			ReadIpmiSensors(m.config.IpmisensorsPath, out)
		}
	}
}

func (m *IpmiCollector) Close() {
	m.init = false
	return
}
