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

// First part contains the code for the general NfsCollector.
// Later, the general NfsCollector is more limited to Nfs3- and Nfs4Collector.

const NFSSTAT_EXEC = `nfsstat`

type NfsCollectorData struct {
	current int64
	last    int64
}

type nfsCollector struct {
	metricCollector
	tags    map[string]string
	version string
	config  struct {
		Nfsstats       string   `json:"nfsstat"`
		ExcludeMetrics []string `json:"exclude_metrics,omitempty"`
	}
	data map[string]NfsCollectorData
}

func (m *nfsCollector) initStats() error {
	cmd := exec.Command(m.config.Nfsstats, `-l`, `--all`)
	cmd.Wait()
	buffer, err := cmd.Output()
	if err == nil {
		for _, line := range strings.Split(string(buffer), "\n") {
			lf := strings.Fields(line)
			if len(lf) != 5 {
				continue
			}
			if lf[1] == m.version {
				name := strings.Trim(lf[3], ":")
				if _, exist := m.data[name]; !exist {
					value, err := strconv.ParseInt(lf[4], 0, 64)
					if err == nil {
						x := m.data[name]
						x.current = value
						x.last = value
						m.data[name] = x
					}
				}
			}
		}
	}
	return err
}

func (m *nfsCollector) updateStats() error {
	cmd := exec.Command(m.config.Nfsstats, `-l`, `--all`)
	cmd.Wait()
	buffer, err := cmd.Output()
	if err == nil {
		for _, line := range strings.Split(string(buffer), "\n") {
			lf := strings.Fields(line)
			if len(lf) != 5 {
				continue
			}
			if lf[1] == m.version {
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

func (m *nfsCollector) MainInit(config json.RawMessage) error {
	m.config.Nfsstats = string(NFSSTAT_EXEC)
	// Read JSON configuration
	if len(config) > 0 {
		err := json.Unmarshal(config, &m.config)
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
	// Check if nfsstat is in executable search path
	_, err := exec.LookPath(m.config.Nfsstats)
	if err != nil {
		return fmt.Errorf("NfsCollector.Init(): Failed to find nfsstat binary '%s': %v", m.config.Nfsstats, err)
	}
	m.data = make(map[string]NfsCollectorData)
	m.initStats()
	m.init = true
	m.parallel = true
	return nil
}

func (m *nfsCollector) Read(interval time.Duration, output chan lp.CCMetric) {
	if !m.init {
		return
	}
	timestamp := time.Now()

	m.updateStats()
	prefix := ""
	switch m.version {
	case "v3":
		prefix = "nfs3"
	case "v4":
		prefix = "nfs4"
	default:
		prefix = "nfs"
	}

	for name, data := range m.data {
		if _, skip := stringArrayContains(m.config.ExcludeMetrics, name); skip {
			continue
		}
		value := data.current - data.last
		y, err := lp.New(fmt.Sprintf("%s_%s", prefix, name), m.tags, m.meta, map[string]interface{}{"value": value}, timestamp)
		if err == nil {
			y.AddMeta("version", m.version)
			output <- y
		}
	}
}

func (m *nfsCollector) Close() {
	m.init = false
}

type Nfs3Collector struct {
	nfsCollector
}

type Nfs4Collector struct {
	nfsCollector
}

func (m *Nfs3Collector) Init(config json.RawMessage) error {
	m.name = "Nfs3Collector"
	m.version = `v3`
	m.setup()
	return m.MainInit(config)
}

func (m *Nfs4Collector) Init(config json.RawMessage) error {
	m.name = "Nfs4Collector"
	m.version = `v4`
	m.setup()
	return m.MainInit(config)
}
