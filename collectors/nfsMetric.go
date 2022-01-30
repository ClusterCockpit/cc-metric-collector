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

type Nfs3Collector struct {
	metricCollector
	tags    map[string]string
	version string
	config  struct {
		Nfsutils       string   `json:"nfsutils"`
		ExcludeMetrics []string `json:"exclude_metrics,omitempty"`
	}
	data map[string]NfsCollectorData
}

type Nfs4Collector struct {
	metricCollector
	tags    map[string]string
	version string
	config  struct {
		Nfsutils       string   `json:"nfsutils"`
		ExcludeMetrics []string `json:"exclude_metrics,omitempty"`
	}
	data map[string]NfsCollectorData
}

func (m *Nfs3Collector) initStats() error {
	cmd := exec.Command(m.config.Nfsutils, `-l`, `-3`)
	cmd.Wait()
	buffer, err := cmd.Output()
	if err == nil {
		for _, line := range strings.Split(string(buffer), "\n") {
			lf := strings.Fields(line)
			if len(lf) != 5 {
				continue
			}
			if lf[1] == m.version { // `v3`
				name := strings.Trim(lf[3], ":")
				if _, exist := m.data[name]; !exist {
					value, err := strconv.ParseInt(lf[4], 0, 64)
					if err == nil {
						x := m.data[name]
						x.current = value
						x.last = 0
						m.data[name] = x
					}
				}
			}
		}
	}
	return err
}

func (m *Nfs4Collector) initStats() error {
	cmd := exec.Command(m.config.Nfsutils, `-l`, `-4`)
	cmd.Wait()
	buffer, err := cmd.Output()
	if err == nil {
		for _, line := range strings.Split(string(buffer), "\n") {
			lf := strings.Fields(line)
			if len(lf) != 5 {
				continue
			}
			if lf[1] == m.version { // `v4`
				name := strings.Trim(lf[3], ":")
				if _, exist := m.data[name]; !exist {
					value, err := strconv.ParseInt(lf[4], 0, 64)
					if err == nil {
						x := m.data[name]
						x.current = value
						x.last = 0
						m.data[name] = x
					}
				}
			}
		}
	}
	return err
}

func (m *Nfs3Collector) updateStats() error {
	cmd := exec.Command(m.config.Nfsutils, `-l`, `-3`)
	cmd.Wait()
	buffer, err := cmd.Output()
	if err == nil {
		for _, line := range strings.Split(string(buffer), "\n") {
			lf := strings.Fields(line)
			if len(lf) != 5 {
				continue
			}
			if lf[1] == m.version { // `v3`
				name := strings.Trim(lf[3], ":")
				if _, exist := m.data[name]; exist {
					value, err := strconv.ParseInt(lf[4], 0, 64)
					if err == nil {
						x := m.data[name]
						x.last = x.current
						x.current = value
						m.data[name] = x
					}
				}
			}
		}
	}
	return err
}

func (m *Nfs4Collector) updateStats() error {
	cmd := exec.Command(m.config.Nfsutils, `-l`, `-4`)
	cmd.Wait()
	buffer, err := cmd.Output()
	if err == nil {
		for _, line := range strings.Split(string(buffer), "\n") {
			lf := strings.Fields(line)
			if len(lf) != 5 {
				continue
			}
			if lf[1] == m.version { // `v4`
				name := strings.Trim(lf[3], ":")
				if _, exist := m.data[name]; exist {
					value, err := strconv.ParseInt(lf[4], 0, 64)
					if err == nil {
						x := m.data[name]
						x.last = x.current
						x.current = value
						m.data[name] = x
					}
				}
			}
		}
	}
	return err
}

func (m *Nfs3Collector) Init(config json.RawMessage) error {
	var err error
	m.name = "Nfs3Collector"
	m.version = `v3`
	m.setup()

	// Set default mmpmon binary
	m.config.Nfsutils = "/usr/sbin/nfsstat"

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
	_, err = exec.LookPath(m.config.Nfsutils)
	if err != nil {
		return fmt.Errorf("NfsCollector.Init(): Failed to find nfsstat binary '%s': %v", m.config.Nfsutils, err)
	}
	m.data = make(map[string]NfsCollectorData)
	m.initStats()
	m.init = true
	return nil
}

func (m *Nfs4Collector) Init(config json.RawMessage) error {
	var err error
	m.name = "Nfs4Collector"
	m.version = `v4`
	m.setup()

	// Set default mmpmon binary
	m.config.Nfsutils = "/usr/sbin/nfsstat"

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
	_, err = exec.LookPath(m.config.Nfsutils)
	if err != nil {
		return fmt.Errorf("NfsCollector.Init(): Failed to find nfsstat binary '%s': %v", m.config.Nfsutils, err)
	}
	m.data = make(map[string]NfsCollectorData)
	m.initStats()
	m.init = true
	return nil
}

func (m *Nfs3Collector) Read(interval time.Duration, output chan lp.CCMetric) {
	if !m.init {
		return
	}
	timestamp := time.Now()

	m.updateStats()

	for name, data := range m.data {
		if _, skip := stringArrayContains(m.config.ExcludeMetrics, name); skip {
			continue
		}
		value := data.current - data.last
		y, err := lp.New(fmt.Sprintf("nfs3_%s", name), m.tags, m.meta, map[string]interface{}{"value": value}, timestamp)
		if err == nil {
			y.AddMeta("version", m.version)
			output <- y
		}
	}
}

func (m *Nfs4Collector) Read(interval time.Duration, output chan lp.CCMetric) {
	if !m.init {
		return
	}
	timestamp := time.Now()

	m.updateStats()

	for name, data := range m.data {
		if _, skip := stringArrayContains(m.config.ExcludeMetrics, name); skip {
			continue
		}
		value := data.current - data.last
		y, err := lp.New(fmt.Sprintf("nfs4_%s", name), m.tags, m.meta, map[string]interface{}{"value": value}, timestamp)
		if err == nil {
			y.AddMeta("version", m.version)
			output <- y
		}
	}
}

func (m *Nfs3Collector) Close() {
	m.init = false
}

func (m *Nfs4Collector) Close() {
	m.init = false
}
