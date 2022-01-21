package collectors

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"os/exec"
	"strings"
	"time"

	lp "github.com/influxdata/line-protocol"
)

const CUSTOMCMDPATH = `/home/unrz139/Work/cc-metric-collector/collectors/custom`

type CustomCmdCollectorConfig struct {
	commands       []string `json:"commands"`
	files          []string `json:"files"`
	ExcludeMetrics []string `json:"exclude_metrics"`
}

type CustomCmdCollector struct {
	MetricCollector
	handler  *lp.MetricHandler
	parser   *lp.Parser
	config   CustomCmdCollectorConfig
	commands []string
	files    []string
}

func (m *CustomCmdCollector) Init(config []byte) error {
	var err error
	m.name = "CustomCmdCollector"
	if len(config) > 0 {
		err = json.Unmarshal(config, &m.config)
		if err != nil {
			log.Print(err.Error())
			return err
		}
	}
	m.setup()
	for _, c := range m.config.commands {
		cmdfields := strings.Fields(c)
		command := exec.Command(cmdfields[0], strings.Join(cmdfields[1:], " "))
		command.Wait()
		_, err = command.Output()
		if err != nil {
			m.commands = append(m.commands, c)
		}
	}
	for _, f := range m.config.files {
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
	m.handler = lp.NewMetricHandler()
	m.parser = lp.NewParser(m.handler)
	m.parser.SetTimeFunc(DefaultTime)
	m.init = true
	return nil
}

var DefaultTime = func() time.Time {
	return time.Unix(42, 0)
}

func (m *CustomCmdCollector) Read(interval time.Duration, out *[]lp.MutableMetric) {
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
			y, err := lp.New(c.Name(), Tags2Map(c), Fields2Map(c), c.Time())
			if err == nil {
				*out = append(*out, y)
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
			y, err := lp.New(f.Name(), Tags2Map(f), Fields2Map(f), f.Time())
			if err == nil {
				*out = append(*out, y)
			}
		}
	}
}

func (m *CustomCmdCollector) Close() {
	m.init = false
}
