package collectors

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os/exec"
	"strconv"
	"strings"
	"time"
	cclog "github.com/ClusterCockpit/cc-metric-collector/pkg/ccLogger"
	lp "github.com/ClusterCockpit/cc-metric-collector/pkg/ccMetric"
)

const IPMISENSORS_PATH = `ipmi-sensors`

type IpmiCollector struct {
	metricCollector
	config struct {
		ExcludeDevices  []string `json:"exclude_devices"`
		IpmitoolPath    string   `json:"ipmitool_path"`
		IpmisensorsPath string   `json:"ipmisensors_path"`
	}
	ipmitool    string
	ipmisensors string
}

func (m *IpmiCollector) Init(config json.RawMessage) error {
	// Check if already initialized
	if m.init {
		return nil
	}

	m.name = "IpmiCollector"
	m.setup()
	m.parallel = true
	m.meta = map[string]string{
		"source": m.name,
		"group":  "IPMI",
	}
	// default path to IPMI tools
	m.config.IpmitoolPath = "ipmitool"
	m.config.IpmisensorsPath = "ipmi-sensors"
	if len(config) > 0 {
		err := json.Unmarshal(config, &m.config)
		if err != nil {
			return err
		}
	}
	// Check if executables ipmitool or ipmisensors are found
	p, err := exec.LookPath(m.config.IpmitoolPath)
	if err == nil {
		m.ipmitool = p
	}
	p, err = exec.LookPath(m.config.IpmisensorsPath)
	if err == nil {
		m.ipmisensors = p
	}
	if len(m.ipmitool) == 0 && len(m.ipmisensors) == 0 {
		return errors.New("no IPMI reader found")
	}
	m.init = true
	return nil
}

func (m *IpmiCollector) readIpmiTool(cmd string, output chan lp.CCMetric) {

	// Setup ipmitool command
	command := exec.Command(cmd, "sensor")
	stdout, _ := command.StdoutPipe()
	errBuf := new(bytes.Buffer)
	command.Stderr = errBuf

	// start command
	if err := command.Start(); err != nil {
		cclog.ComponentError(
			m.name,
			fmt.Sprintf("readIpmiTool(): Failed to start command \"%s\": %v", command.String(), err),
		)
		return
	}

	// Read command output
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		lv := strings.Split(scanner.Text(), "|")
		if len(lv) < 3 {
			continue
		}
		v, err := strconv.ParseFloat(strings.TrimSpace(lv[1]), 64)
		if err == nil {
			name := strings.ToLower(strings.Replace(strings.TrimSpace(lv[0]), " ", "_", -1))
			unit := strings.TrimSpace(lv[2])
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

	// Wait for command end
	if err := command.Wait(); err != nil {
		errMsg, _ := io.ReadAll(errBuf)
		cclog.ComponentError(
			m.name,
			fmt.Sprintf("readIpmiTool(): Failed to wait for the end of command \"%s\": %v\n", command.String(), err),
			fmt.Sprintf("readIpmiTool(): command stderr: \"%s\"\n", string(errMsg)),
		)
		return
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

	// Check if already initialized
	if !m.init {
		return
	}

	if len(m.config.IpmitoolPath) > 0 {
		m.readIpmiTool(m.config.IpmitoolPath, output)
	} else if len(m.config.IpmisensorsPath) > 0 {
		m.readIpmiSensors(m.config.IpmisensorsPath, output)
	}
}

func (m *IpmiCollector) Close() {
	m.init = false
}
