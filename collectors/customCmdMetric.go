package collectors

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"os/exec"
	"strings"
	"time"

<<<<<<< HEAD
	lp "github.com/influxdata/line-protocol"
=======
	ccmetric "github.com/ClusterCockpit/cc-metric-collector/internal/ccMetric"
	lp "github.com/ClusterCockpit/cc-metric-collector/internal/ccMetric"
	influx "github.com/influxdata/line-protocol"
>>>>>>> develop
)

const CUSTOMCMDPATH = `/home/unrz139/Work/cc-metric-collector/collectors/custom`

type CustomCmdCollectorConfig struct {
	Commands       []string `json:"commands"`
	Files          []string `json:"files"`
	ExcludeMetrics []string `json:"exclude_metrics"`
}

type CustomCmdCollector struct {
	metricCollector
	handler  *influx.MetricHandler
	parser   *influx.Parser
	config   CustomCmdCollectorConfig
	commands []string
	files    []string
}

func (m *CustomCmdCollector) Init(config json.RawMessage) error {
	var err error
	m.name = "CustomCmdCollector"
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
		if err != nil {
			m.commands = append(m.commands, c)
		}
	}
	for _, f := range m.config.Files {
		_, err = ioutil.ReadFile(f)
		if err == nil {
			m.files = append(m.files, f)
		} else {
			log.Print(err.Error())
			continue
		}
	}
	if len(m.files) == 0 && len(m.commands) == 0 {
		return errors.New("No metrics to collect")
	}
	m.handler = influx.NewMetricHandler()
	m.parser = influx.NewParser(m.handler)
	m.parser.SetTimeFunc(DefaultTime)
	m.init = true
	return nil
}

var DefaultTime = func() time.Time {
	return time.Unix(42, 0)
}

func (m *CustomCmdCollector) Read(interval time.Duration, output chan lp.CCMetric) {
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
		cmdmetrics, err := m.parser.Parse(stdout)
		if err != nil {
			log.Print(err)
			continue
		}
		for _, c := range cmdmetrics {
			_, skip := stringArrayContains(m.config.ExcludeMetrics, c.Name())
			if skip {
				continue
			}

			y := ccmetric.FromInfluxMetric(c)
			if err == nil {
				output <- y
			}
		}
	}
	for _, file := range m.files {
		buffer, err := ioutil.ReadFile(file)
		if err != nil {
			log.Print(err)
			return
		}
		fmetrics, err := m.parser.Parse(buffer)
		if err != nil {
			log.Print(err)
			continue
		}
		for _, f := range fmetrics {
			_, skip := stringArrayContains(m.config.ExcludeMetrics, f.Name())
			if skip {
				continue
			}
			y := ccmetric.FromInfluxMetric(f)
			if err == nil {
				output <- y
			}
		}
	}
}

func (m *CustomCmdCollector) Close() {
	m.init = false
}
