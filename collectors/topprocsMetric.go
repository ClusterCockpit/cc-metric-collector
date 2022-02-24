package collectors

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"

	lp "github.com/ClusterCockpit/cc-metric-collector/internal/ccMetric"
)

const MAX_NUM_PROCS = 10
const DEFAULT_NUM_PROCS = 2

type TopProcsCollectorConfig struct {
	Num_procs int `json:"num_procs"`
}

type TopProcsCollector struct {
	metricCollector
	tags   map[string]string
	config TopProcsCollectorConfig
}

func (m *TopProcsCollector) Init(config json.RawMessage) error {
	var err error
	m.name = "TopProcsCollector"
	m.tags = map[string]string{"type": "node"}
	m.meta = map[string]string{"source": m.name, "group": "TopProcs"}
	if len(config) > 0 {
		err = json.Unmarshal(config, &m.config)
		if err != nil {
			return err
		}
	} else {
		m.config.Num_procs = int(DEFAULT_NUM_PROCS)
	}
	if m.config.Num_procs <= 0 || m.config.Num_procs > MAX_NUM_PROCS {
		return errors.New(fmt.Sprintf("num_procs option must be set in 'topprocs' config (range: 1-%d)", MAX_NUM_PROCS))
	}
	m.setup()
	command := exec.Command("ps", "-Ao", "comm", "--sort=-pcpu")
	command.Wait()
	_, err = command.Output()
	if err != nil {
		return errors.New("Failed to execute command")
	}
	m.init = true
	return nil
}

func (m *TopProcsCollector) Read(interval time.Duration, output chan lp.CCMetric) {
	if !m.init {
		return
	}
	command := exec.Command("ps", "-Ao", "comm", "--sort=-pcpu")
	command.Wait()
	stdout, err := command.Output()
	if err != nil {
		log.Print(m.name, err)
		return
	}

	lines := strings.Split(string(stdout), "\n")
	for i := 1; i < m.config.Num_procs+1; i++ {
		name := fmt.Sprintf("topproc%d", i)
		y, err := lp.New(name, m.tags, m.meta, map[string]interface{}{"value": string(lines[i])}, time.Now())
		if err == nil {
			output <- y
		}
	}
}

func (m *TopProcsCollector) Close() {
	m.init = false
}
