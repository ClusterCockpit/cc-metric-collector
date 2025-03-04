package collectors

import (
	"encoding/json"
	"errors"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	lp "github.com/ClusterCockpit/cc-lib/ccMessage"
	influx "github.com/influxdata/line-protocol"
)

const CUSTOMCMDPATH = `/home/unrz139/Work/cc-metric-collector/collectors/custom`

type CustomCmdCollectorConfig struct {
	Commands           []string `json:"commands"`
	Files              []string `json:"files"`
	ExcludeMetrics     []string `json:"exclude_metrics"`
	OnlyMetrics        []string `json:"only_metrics,omitempty"`
	SendAbsoluteValues *bool    `json:"send_abs_values,omitempty"`
	SendDiffValues     *bool    `json:"send_diff_values,omitempty"`
	SendDerivedValues  *bool    `json:"send_derived_values,omitempty"`
}

// set default values: send_abs_values: true, send_diff_values: false, send_derived_values: false.
func (cfg *CustomCmdCollectorConfig) AbsValues() bool {
	if cfg.SendAbsoluteValues == nil {
		return true
	}
	return *cfg.SendAbsoluteValues
}
func (cfg *CustomCmdCollectorConfig) DiffValues() bool {
	if cfg.SendDiffValues == nil {
		return false
	}
	return *cfg.SendDiffValues
}
func (cfg *CustomCmdCollectorConfig) DerivedValues() bool {
	if cfg.SendDerivedValues == nil {
		return false
	}
	return *cfg.SendDerivedValues
}

type CustomCmdCollector struct {
	metricCollector
	handler   *influx.MetricHandler
	parser    *influx.Parser
	config    CustomCmdCollectorConfig
	commands  []string
	files     []string
	oldValues map[string]float64
}

func (m *CustomCmdCollector) shouldOutput(metricName string) bool {
	if len(m.config.OnlyMetrics) > 0 {
		for _, name := range m.config.OnlyMetrics {
			if name == metricName {
				return true
			}
		}
		return false
	}
	for _, name := range m.config.ExcludeMetrics {
		if name == metricName {
			return false
		}
	}
	return true
}

func (m *CustomCmdCollector) Init(config json.RawMessage) error {
	var err error
	m.name = "CustomCmdCollector"
	m.parallel = true
	m.meta = map[string]string{"source": m.name, "group": "Custom"}
	if len(config) > 0 {
		err = json.Unmarshal(config, &m.config)
		if err != nil {
			log.Print(err.Error())
			return err
		}
	}
	m.setup()
	for _, c := range m.config.Commands {
		cmdfields := strings.Fields(c)
		command := exec.Command(cmdfields[0], strings.Join(cmdfields[1:], " "))
		command.Wait()
		_, err = command.Output()
		if err == nil {
			m.commands = append(m.commands, c)
		}
	}
	for _, f := range m.config.Files {
		_, err = os.ReadFile(f)
		if err == nil {
			m.files = append(m.files, f)
		} else {
			log.Print(err.Error())
			continue
		}
	}
	if len(m.files) == 0 && len(m.commands) == 0 {
		return errors.New("no metrics to collect")
	}
	m.handler = influx.NewMetricHandler()
	m.parser = influx.NewParser(m.handler)
	m.parser.SetTimeFunc(DefaultTime)
	m.oldValues = make(map[string]float64)
	m.init = true
	return nil
}

var DefaultTime = func() time.Time {
	return time.Unix(42, 0)
}

// copyMeta creates a deep copy of a map[string]string.
func copyMeta(orig map[string]string) map[string]string {
	newMeta := make(map[string]string)
	for k, v := range orig {
		newMeta[k] = v
	}
	return newMeta
}

// getMetricValueFromMsg extracts the numeric "value" field from a CCMessage.
func getMetricValueFromMsg(msg lp.CCMessage) (float64, bool) {
	fields := msg.Fields()
	if v, ok := fields["value"]; ok {
		if num, ok := v.(float64); ok {
			return num, true
		}
	}
	return 0, false
}

// processMetric processes a single metric:
// - If send_abs_values is enabled, sends the absolute metric.
// - If send_diff_values is enabled and a previous value exists, sends the diff value under the name "<base>_diff".
// - If send_derived_values is enabled and a previous value exists, sends the derived rate as "<base>_rate".
func (m *CustomCmdCollector) processMetric(msg lp.CCMessage, interval time.Duration, output chan lp.CCMessage) {
	name := msg.Name()
	if !m.shouldOutput(name) {
		return
	}
	if m.config.AbsValues() {
		output <- msg
	}
	val, ok := getMetricValueFromMsg(msg)
	if !ok {
		return
	}
	if prev, exists := m.oldValues[name]; exists {
		diff := val - prev
		if m.config.DiffValues() && m.shouldOutput(name+"_diff") {
			diffMsg, err := lp.NewMessage(name+"_diff", msg.Tags(), msg.Meta(), map[string]interface{}{"value": diff}, time.Now())
			if err == nil {
				output <- diffMsg
			}
		}
		if m.config.DerivedValues() && m.shouldOutput(name+"_rate") {
			derived := diff / interval.Seconds()
			newMeta := copyMeta(msg.Meta())
			unit := newMeta["unit"]
			if unit == "" {
				if tagUnit, ok := msg.Tags()["unit"]; ok {
					unit = tagUnit
					newMeta["unit"] = unit
				}
			}
			if unit != "" && !strings.HasSuffix(unit, "/s") {
				newMeta["unit"] = unit + "/s"
			}
			derivedMsg, err := lp.NewMessage(name+"_rate", msg.Tags(), newMeta, map[string]interface{}{"value": derived}, time.Now())
			if err == nil {
				output <- derivedMsg
			}
		}
	}
	m.oldValues[name] = val
}

// Read processes commands and files, parses their output, and processes each metric.
func (m *CustomCmdCollector) Read(interval time.Duration, output chan lp.CCMessage) {
	if !m.init {
		return
	}
	for _, cmd := range m.commands {
		cmdfields := strings.Fields(cmd)
		command := exec.Command(cmdfields[0], strings.Join(cmdfields[1:], " "))
		command.Wait()
		stdout, err := command.Output()
		if err != nil {
			log.Print(err)
			continue
		}
		metrics, err := m.parser.Parse(stdout)
		if err != nil {
			log.Print(err)
			continue
		}
		for _, met := range metrics {
			msg := lp.FromInfluxMetric(met)
			m.processMetric(msg, interval, output)
		}
	}
	for _, file := range m.files {
		buffer, err := os.ReadFile(file)
		if err != nil {
			log.Print(err)
			continue
		}
		metrics, err := m.parser.Parse(buffer)
		if err != nil {
			log.Print(err)
			continue
		}
		for _, met := range metrics {
			msg := lp.FromInfluxMetric(met)
			m.processMetric(msg, interval, output)
		}
	}
}

func (m *CustomCmdCollector) Close() {
	m.init = false
}
