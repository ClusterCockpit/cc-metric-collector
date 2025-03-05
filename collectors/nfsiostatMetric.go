package collectors

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	lp "github.com/ClusterCockpit/cc-lib/ccMessage"
	cclog "github.com/ClusterCockpit/cc-metric-collector/pkg/ccLogger"
)

// NfsIOStatCollectorConfig holds configuration options for the nfsiostat collector.
type NfsIOStatCollectorConfig struct {
	ExcludeMetrics          []string `json:"exclude_metrics,omitempty"`
	OnlyMetrics             []string `json:"only_metrics,omitempty"`
	ExcludeFilesystem       []string `json:"exclude_filesystem,omitempty"`
	UseServerAddressAsSType bool     `json:"use_server_as_stype,omitempty"`
	SendAbsoluteValues      bool     `json:"send_abs_values"`
	SendDerivedValues       bool     `json:"send_derived_values"`
}

// NfsIOStatCollector reads NFS I/O statistics from /proc/self/mountstats.
type NfsIOStatCollector struct {
	metricCollector
	config        NfsIOStatCollectorConfig
	meta          map[string]string
	tags          map[string]string
	data          map[string]map[string]int64 // previous values per filesystem
	key           string                      // "server" or "mntpoint"
	lastTimestamp time.Time
}

// Regular expressions to parse mount info and byte statistics.
var deviceRegex = regexp.MustCompile(`device (?P<server>[^ ]+) mounted on (?P<mntpoint>[^ ]+) with fstype nfs(?P<version>\d*) statvers=[\d\.]+`)
var bytesRegex = regexp.MustCompile(`\s+bytes:\s+(?P<nread>[^ ]+) (?P<nwrite>[^ ]+) (?P<dread>[^ ]+) (?P<dwrite>[^ ]+) (?P<nfsread>[^ ]+) (?P<nfswrite>[^ ]+) (?P<pageread>[^ ]+) (?P<pagewrite>[^ ]+)`)

// resolve_regex_fields extracts named regex groups from a string.
func resolve_regex_fields(s string, regex *regexp.Regexp) map[string]string {
	fields := make(map[string]string)
	groups := regex.SubexpNames()
	for _, match := range regex.FindAllStringSubmatch(s, -1) {
		for groupIdx, group := range match {
			if len(groups[groupIdx]) > 0 {
				fields[groups[groupIdx]] = group
			}
		}
	}
	return fields
}

// shouldOutput returns true if a base metric (without prefix) is allowed.
func (m *NfsIOStatCollector) shouldOutput(metricName string) bool {
	if len(m.config.OnlyMetrics) > 0 {
		for _, n := range m.config.OnlyMetrics {
			if n == metricName {
				return true
			}
		}
		return false
	}
	for _, n := range m.config.ExcludeMetrics {
		if n == metricName {
			return false
		}
	}
	return true
}

func (m *NfsIOStatCollector) Init(config json.RawMessage) error {
	var err error
	m.name = "NfsIOStatCollector"
	m.setup()
	m.parallel = true
	m.meta = map[string]string{"source": m.name, "group": "NFS", "unit": "bytes"}
	m.tags = map[string]string{"type": "node"}
	// Default: use_server_as_stype is false.
	m.config.UseServerAddressAsSType = false
	// Defaults for absolute and derived values.
	m.config.SendAbsoluteValues = false
	m.config.SendDerivedValues = true
	if len(config) > 0 {
		if err = json.Unmarshal(config, &m.config); err != nil {
			cclog.ComponentError(m.name, "Error reading config:", err.Error())
			return err
		}
	}
	m.key = "mntpoint"
	if m.config.UseServerAddressAsSType {
		m.key = "server"
	}
	m.data = m.readNfsiostats()
	m.lastTimestamp = time.Now()
	m.init = true
	return nil
}

func (m *NfsIOStatCollector) readNfsiostats() map[string]map[string]int64 {
	data := make(map[string]map[string]int64)
	filename := "/proc/self/mountstats"
	stats, err := os.ReadFile(filename)
	if err != nil {
		return data
	}

	lines := strings.Split(string(stats), "\n")
	var current map[string]string = nil
	for _, l := range lines {
		// Check for a device line.
		dev := resolve_regex_fields(l, deviceRegex)
		if len(dev) > 0 {
			if _, ok := stringArrayContains(m.config.ExcludeFilesystem, dev[m.key]); !ok {
				current = dev
				if len(current["version"]) == 0 {
					current["version"] = "3"
				}
			} else {
				current = nil
			}
		}
		if current != nil {
			// Parse byte statistics line.
			bytes := resolve_regex_fields(l, bytesRegex)
			if len(bytes) > 0 {
				data[current[m.key]] = make(map[string]int64)
				for name, sval := range bytes {
					if _, ok := stringArrayContains(m.config.ExcludeMetrics, name); ok {
						continue
					}
					if len(m.config.OnlyMetrics) > 0 {
						found := false
						for _, metric := range m.config.OnlyMetrics {
							if metric == name {
								found = true
								break
							}
						}
						if !found {
							continue
						}
					}
					val, err := strconv.ParseInt(sval, 10, 64)
					if err == nil {
						data[current[m.key]][name] = val
					}
				}
				current = nil
			}
		}
	}
	return data
}

func (m *NfsIOStatCollector) Read(interval time.Duration, output chan lp.CCMessage) {
	now := time.Now()
	timeDiff := now.Sub(m.lastTimestamp).Seconds()
	m.lastTimestamp = now

	newdata := m.readNfsiostats()

	for mntpoint, values := range newdata {
		if _, ok := stringArrayContains(m.config.ExcludeFilesystem, mntpoint); ok {
			continue
		}
		for name, newVal := range values {
			baseName := name // Base metric name.
			if m.config.SendAbsoluteValues && m.shouldOutput(baseName) {
				msg, err := lp.NewMessage(fmt.Sprintf("nfsio_%s", baseName), m.tags, m.meta, map[string]interface{}{"value": newVal}, now)
				if err == nil {
					msg.AddTag("stype", "filesystem")
					msg.AddTag("stype-id", mntpoint)
					output <- msg
				}
			}
			if m.config.SendDerivedValues {
				if old, ok := m.data[mntpoint][name]; ok {
					rate := float64(newVal-old) / timeDiff
					if m.shouldOutput(baseName) {
						msg, err := lp.NewMessage(fmt.Sprintf("nfsio_%s_bw", baseName), m.tags, m.meta, map[string]interface{}{"value": rate}, now)
						if err == nil {
							if strings.HasPrefix(name, "page") {
								msg.AddMeta("unit", "4K_pages/s")
							} else {
								msg.AddMeta("unit", "bytes/sec")
							}
							msg.AddTag("stype", "filesystem")
							msg.AddTag("stype-id", mntpoint)
							output <- msg
						}
					}
				}
			}
			if m.data[mntpoint] == nil {
				m.data[mntpoint] = make(map[string]int64)
			}
			m.data[mntpoint][name] = newVal
		}
	}
	// Remove mountpoints that no longer exist.
	for mntpoint := range m.data {
		found := false
		for newMnt := range newdata {
			if newMnt == mntpoint {
				found = true
				break
			}
		}
		if !found {
			m.data[mntpoint] = nil
		}
	}
}

func (m *NfsIOStatCollector) Close() {
	m.init = false
}
