package collectors

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	lp "github.com/ClusterCockpit/cc-lib/ccMessage"
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
		Nfsstats           string   `json:"nfsstat"`
		ExcludeMetrics     []string `json:"exclude_metrics,omitempty"`
		OnlyMetrics        []string `json:"only_metrics,omitempty"`
		SendAbsoluteValues bool     `json:"send_abs_values,omitempty"`
		SendDiffValues     bool     `json:"send_diff_values,omitempty"`
		SendDerivedValues  bool     `json:"send_derived_values,omitempty"`
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
				// Use the base metric name (without prefix) for filtering.
				name := strings.Trim(lf[3], ":")
				if _, exist := m.data[name]; !exist {
					value, err := strconv.ParseInt(lf[4], 0, 64)
					if err == nil {
						m.data[name] = NfsCollectorData{current: value, last: value}
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

// shouldOutput returns true if a metric (base name + variant) should be forwarded.
// The variant is "" for absolute, "_diff" for diff, and "_rate" for derived.
// If only_metrics is set, only the metric names that exactly match (base+variant) are forwarded.
func (m *nfsCollector) shouldOutput(baseName, variant string) bool {
	finalName := baseName + variant
	if len(m.config.OnlyMetrics) > 0 {
		for _, n := range m.config.OnlyMetrics {
			if n == finalName {
				return true
			}
		}
		return false
	}
	for _, n := range m.config.ExcludeMetrics {
		if n == finalName {
			return false
		}
	}
	return true
}

func (m *nfsCollector) MainInit(config json.RawMessage) error {
	m.config.Nfsstats = NFSSTAT_EXEC
	if len(config) > 0 {
		err := json.Unmarshal(config, &m.config)
		if err != nil {
			return err
		}
	}
	// For backwards compatibility, send_abs_values defaults to true.
	if !m.config.SendAbsoluteValues {
		m.config.SendAbsoluteValues = true
	}
	// Diff and derived default to false if not specified.
	m.meta = map[string]string{"source": m.name, "group": "NFS"}
	m.tags = map[string]string{"type": "node"}
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

func (m *nfsCollector) Read(interval time.Duration, output chan lp.CCMessage) {
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
		// For each metric, apply filtering based on the final output name.
		// Absolute metric: final name = prefix + "_" + name
		if m.config.SendAbsoluteValues && m.shouldOutput(name, "") {
			absMsg, err := lp.NewMessage(fmt.Sprintf("%s_%s", prefix, name), m.tags, m.meta, map[string]interface{}{"value": data.current}, timestamp)
			if err == nil {
				absMsg.AddMeta("version", m.version)
				output <- absMsg
			}
		}
		if m.config.SendDiffValues && m.shouldOutput(name, "_diff") {
			diff := data.current - data.last
			diffMsg, err := lp.NewMessage(fmt.Sprintf("%s_%s_diff", prefix, name), m.tags, m.meta, map[string]interface{}{"value": diff}, timestamp)
			if err == nil {
				diffMsg.AddMeta("version", m.version)
				output <- diffMsg
			}
		}
		if m.config.SendDerivedValues && m.shouldOutput(name, "_rate") {
			diff := data.current - data.last
			rate := float64(diff) / interval.Seconds()
			derivedMsg, err := lp.NewMessage(fmt.Sprintf("%s_%s_rate", prefix, name), m.tags, m.meta, map[string]interface{}{"value": rate}, timestamp)
			if err == nil {
				derivedMsg.AddMeta("version", m.version)
				output <- derivedMsg
			}
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
	m.version = "v3"
	m.setup()
	return m.MainInit(config)
}

func (m *Nfs4Collector) Init(config json.RawMessage) error {
	m.name = "Nfs4Collector"
	m.version = "v4"
	m.setup()
	return m.MainInit(config)
}
