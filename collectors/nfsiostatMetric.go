package collectors

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	cclog "github.com/ClusterCockpit/cc-metric-collector/pkg/ccLogger"
	lp "github.com/ClusterCockpit/cc-metric-collector/pkg/ccMetric"
)

// These are the fields we read from the JSON configuration
type NfsIOStatCollectorConfig struct {
	ExcludeMetrics          []string `json:"exclude_metrics,omitempty"`
	ExcludeFilesystem       []string `json:"exclude_filesystem,omitempty"`
	UseServerAddressAsSType bool     `json:"use_server_as_stype,omitempty"`
}

// This contains all variables we need during execution and the variables
// defined by metricCollector (name, init, ...)
type NfsIOStatCollector struct {
	metricCollector
	config NfsIOStatCollectorConfig // the configuration structure
	meta   map[string]string        // default meta information
	tags   map[string]string        // default tags
	data   map[string]map[string]int64
}

var deviceRegex = regexp.MustCompile(`device (?P<server>[^ ]+) mounted on (?P<mntpoint>[^ ]+) with fstype nfs(?P<version>\d*) statvers=[\d\.]+`)
var bytesRegex = regexp.MustCompile(`\s+bytes:\s+(?P<nread>[^ ]+) (?P<nwrite>[^ ]+) (?P<dread>[^ ]+) (?P<dwrite>[^ ]+) (?P<nfsread>[^ ]+) (?P<nfswrite>[^ ]+) (?P<pageread>[^ ]+) (?P<pagewrite>[^ ]+)`)

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

func (m *NfsIOStatCollector) readNfsiostats() map[string]map[string]int64 {
	data := make(map[string]map[string]int64)
	filename := "mountstats.txt"
	// filename := "/proc/self/mountstats"
	cclog.ComponentDebug(m.name, "Reading", filename)
	stats, err := os.ReadFile(filename)
	if err != nil {
		cclog.ComponentError(m.name, "Failed reading", filename)
		return data
	}
	key := "mntpoint"
	if m.config.UseServerAddressAsSType {
		key = "server"
	}

	lines := strings.Split(string(stats), "\n")
	var current map[string]string = nil
	for _, l := range lines {
		// fmt.Println(l)
		dev := resolve_regex_fields(l, deviceRegex)
		if current == nil && len(dev) > 0 {
			if _, ok := stringArrayContains(m.config.ExcludeFilesystem, dev[key]); !ok {
				current = dev
				cclog.ComponentDebug(m.name, "Found device", current[key])
				if len(current["version"]) == 0 {
					current["version"] = "3"
					cclog.ComponentDebug(m.name, "Sanitize version to ", current["version"])
				}
			}
		}
		bytes := resolve_regex_fields(l, bytesRegex)
		if len(bytes) > 0 && len(current) > 0 {
			cclog.ComponentDebug(m.name, "Found bytes for", current[key])
			data[current[key]] = make(map[string]int64)
			for name, sval := range bytes {
				if _, ok := stringArrayContains(m.config.ExcludeMetrics, name); !ok {
					val, err := strconv.ParseInt(sval, 10, 64)
					if err == nil {
						data[current[key]][name] = val
					}
				}

			}
			current = nil
		}
	}
	return data
}

func (m *NfsIOStatCollector) Init(config json.RawMessage) error {
	var err error = nil
	m.name = "NfsIOStatCollector"
	m.setup()
	m.parallel = true
	m.meta = map[string]string{"source": m.name, "group": "NFS", "unit": "bytes"}
	m.tags = map[string]string{"type": "node"}
	if len(config) > 0 {
		err = json.Unmarshal(config, &m.config)
		if err != nil {
			cclog.ComponentError(m.name, "Error reading config:", err.Error())
			return err
		}
	}
	m.data = m.readNfsiostats()
	m.init = true
	return err
}

func (m *NfsIOStatCollector) Read(interval time.Duration, output chan lp.CCMetric) {
	timestamp := time.Now()

	newdata := m.readNfsiostats()
	for mntpoint, values := range newdata {
		if old, ok := m.data[mntpoint]; ok {
			for i := range values {
				x := values[i] - old[i]
				y, err := lp.New(fmt.Sprintf("nfsio_%s", i), m.tags, m.meta, map[string]interface{}{"value": x}, timestamp)
				if err == nil {
					if strings.HasPrefix(i, "page") {
						y.AddMeta("unit", "4K_Pages")
					}
					y.AddTag("stype", "filesystem")
					y.AddTag("stype-id", mntpoint)
					// Send it to output channel
					output <- y
				}
				old[i] = values[i]
			}
		} else {
			m.data[mntpoint] = values
		}
	}

}

func (m *NfsIOStatCollector) Close() {
	// Unset flag
	m.init = false
}
