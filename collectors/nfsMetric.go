package collectors

import (
	"encoding/json"
	"fmt"
	"log"
	//	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	lp "github.com/ClusterCockpit/cc-metric-collector/internal/ccMetric"
)

type NfsCollectorData struct {
	current int64
	last    int64
}

type NfsCollector struct {
	metricCollector
	tags   map[string]string
	config struct {
		nfsutils       string   `json:"nfsutils"`
		excludeMetrics []string `json:"exclude_metrics,omitempty"`
	}
	data map[string]map[string]NfsCollectorData
}

func (m *NfsCollector) initStats() error {
	cmd := exec.Command(m.config.nfsutils, "-l")
	cmd.Wait()
	buffer, err := cmd.Output()
	if err == nil {
		for _, line := range strings.Split(string(buffer), "\n") {
			lf := strings.Fields(line)
			if len(lf) != 5 {
				continue
			}
			if _, exist := m.data[lf[1]]; !exist {
				m.data[lf[1]] = make(map[string]NfsCollectorData)
			}
			name := strings.Trim(lf[3], ":")
			if _, exist := m.data[lf[1]][name]; !exist {
				value, err := strconv.ParseInt(lf[4], 0, 64)
				if err == nil {
					x := m.data[lf[1]][name]
					x.current = value
					x.last = 0
					m.data[lf[1]][name] = x
				}
			}
		}
	}
	return err
}

func (m *NfsCollector) updateStats() error {
	cmd := exec.Command(m.config.nfsutils, "-l")
	cmd.Wait()
	buffer, err := cmd.Output()
	if err == nil {
		for _, line := range strings.Split(string(buffer), "\n") {
			lf := strings.Fields(line)
			if len(lf) != 5 {
				continue
			}
			if _, exist := m.data[lf[1]]; !exist {
				m.data[lf[1]] = make(map[string]NfsCollectorData)
			}
			name := strings.Trim(lf[3], ":")
			if _, exist := m.data[lf[1]][name]; exist {
				value, err := strconv.ParseInt(lf[4], 0, 64)
				if err == nil {
					x := m.data[lf[1]][name]
					x.last = x.current
					x.current = value
					m.data[lf[1]][name] = x
				}
			}
		}
	}
	return err
}

func (m *NfsCollector) Init(config json.RawMessage) error {
	var err error
	m.name = "NfsCollector"
	m.setup()

	// Set default mmpmon binary
	m.config.nfsutils = "/usr/sbin/nfsstat"

	// Read JSON configuration
	if len(config) > 0 {
		err = json.Unmarshal(config, &m.config)
		if err != nil {
			log.Print(err.Error())
			return err
		}
	}
	m.meta = map[string]string{
		"source": m.name,
		"group":  "NFS",
	}
	m.tags = map[string]string{
		"type": "node",
	}
	// Check if mmpmon is in executable search path
	_, err = exec.LookPath(m.config.nfsutils)
	if err != nil {
		return fmt.Errorf("NfsCollector.Init(): Failed to find nfsstat binary '%s': %v", m.config.nfsutils, err)
	}
	m.data = make(map[string]map[string]NfsCollectorData)
	m.initStats()
	m.init = true
	return nil
}

func (m *NfsCollector) Read(interval time.Duration, output chan lp.CCMetric) {
	if !m.init {
		return
	}
	timestamp := time.Now()

	m.updateStats()

	for version, metrics := range m.data {
		for name, data := range metrics {
			if _, skip := stringArrayContains(m.config.excludeMetrics, name); skip {
				continue
			}
			value := data.current - data.last
			y, err := lp.New(fmt.Sprintf("nfs_%s", name), m.tags, m.meta, map[string]interface{}{"value": value}, timestamp)
			if err == nil {
				y.AddMeta("version", version)
				output <- y
			}
		}
	}
}

func (m *NfsCollector) Close() {
	m.init = false
}
