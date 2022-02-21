package collectors

import (
	"encoding/json"
	"errors"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	lp "github.com/ClusterCockpit/cc-metric-collector/internal/ccMetric"
)

const IPMITOOL_PATH = `ipmitool`
const IPMISENSORS_PATH = `ipmi-sensors`

type IpmiCollectorConfig struct {
	ExcludeDevices  []string `json:"exclude_devices"`
	IpmitoolPath    string   `json:"ipmitool_path"`
	IpmisensorsPath string   `json:"ipmisensors_path"`
}

type IpmiCollector struct {
	metricCollector
	//tags        map[string]string
	//matches     map[string]string
	config      IpmiCollectorConfig
	ipmitool    string
	ipmisensors string
}

func (m *IpmiCollector) Init(config json.RawMessage) error {
	m.name = "IpmiCollector"
	m.setup()
	m.meta = map[string]string{"source": m.name, "group": "IPMI"}
	m.config.IpmitoolPath = string(IPMITOOL_PATH)
	m.config.IpmisensorsPath = string(IPMISENSORS_PATH)
	m.ipmitool = ""
	m.ipmisensors = ""
	if len(config) > 0 {
		err := json.Unmarshal(config, &m.config)
		if err != nil {
			return err
		}
	}
	p, err := exec.LookPath(m.config.IpmitoolPath)
	if err == nil {
		m.ipmitool = p
	}
	p, err = exec.LookPath(m.config.IpmisensorsPath)
	if err == nil {
		m.ipmisensors = p
	}
	if len(m.ipmitool) == 0 && len(m.ipmisensors) == 0 {
		return errors.New("No IPMI reader found")
	}
	m.init = true
	return nil
}

func (m *IpmiCollector) readIpmiTool(cmd string, output chan lp.CCMetric) {
	command := exec.Command(cmd, "sensor")
	command.Wait()
	stdout, err := command.Output()
	if err != nil {
		log.Print(err)
		return
	}

	ll := strings.Split(string(stdout), "\n")

	for _, line := range ll {
		lv := strings.Split(line, "|")
		if len(lv) < 3 {
			continue
		}
		v, err := strconv.ParseFloat(strings.Trim(lv[1], " "), 64)
		if err == nil {
			name := strings.ToLower(strings.Replace(strings.Trim(lv[0], " "), " ", "_", -1))
			unit := strings.Trim(lv[2], " ")
			if unit == "Volts" {
				unit = "Volts"
			} else if unit == "degrees C" {
				unit = "degC"
			} else if unit == "degrees F" {
				unit = "degF"
			} else if unit == "Watts" {
				unit = "Watts"
			}

			y, err := lp.New(name, map[string]string{"type": "node"}, m.meta, map[string]interface{}{"value": v}, time.Now())
			if err == nil {
				y.AddMeta("unit", unit)
				output <- y
			}
		}
	}
}

func (m *IpmiCollector) readIpmiSensors(cmd string, output chan lp.CCMetric) {

	command := exec.Command(cmd, "--comma-separated-output", "--sdr-cache-recreate")
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
				y, err := lp.New(name, map[string]string{"type": "node"}, m.meta, map[string]interface{}{"value": v}, time.Now())
				if err == nil {
					if len(lv) > 4 {
						y.AddMeta("unit", lv[4])
					}
					output <- y
				}
			}
		}
	}
}

func (m *IpmiCollector) Read(interval time.Duration, output chan lp.CCMetric) {
	if len(m.config.IpmitoolPath) > 0 {
		_, err := os.Stat(m.config.IpmitoolPath)
		if err == nil {
			m.readIpmiTool(m.config.IpmitoolPath, output)
		}
	} else if len(m.config.IpmisensorsPath) > 0 {
		_, err := os.Stat(m.config.IpmisensorsPath)
		if err == nil {
			m.readIpmiSensors(m.config.IpmisensorsPath, output)
		}
	}
}

func (m *IpmiCollector) Close() {
	m.init = false
}
