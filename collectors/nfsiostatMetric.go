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
	config NfsIOStatCollectorConfig    // the configuration structure
	meta   map[string]string           // default meta information
	tags   map[string]string           // default tags
	data   map[string]map[string]int64 // data storage for difference calculation
	key    string                      // which device info should be used as subtype ID? 'server' or 'mntpoint', see NfsIOStatCollectorConfig.UseServerAddressAsSType
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
	filename := "/proc/self/mountstats"
	stats, err := os.ReadFile(filename)
	if err != nil {
		return data
	}

	lines := strings.Split(string(stats), "\n")
	var current map[string]string = nil
	for _, l := range lines {
		// Is this a device line with mount point, remote target and NFS version?
		dev := resolve_regex_fields(l, deviceRegex)
		if len(dev) > 0 {
			if _, ok := stringArrayContains(m.config.ExcludeFilesystem, dev[m.key]); !ok {
				current = dev
				if len(current["version"]) == 0 {
					current["version"] = "3"
				}
			}
		}

		if len(current) > 0 {
			// Byte line parsing (if found the device for it)
			bytes := resolve_regex_fields(l, bytesRegex)
			if len(bytes) > 0 {
				data[current[m.key]] = make(map[string]int64)
				for name, sval := range bytes {
					if _, ok := stringArrayContains(m.config.ExcludeMetrics, name); !ok {
						val, err := strconv.ParseInt(sval, 10, 64)
						if err == nil {
							data[current[m.key]][name] = val
						}
					}

				}
				current = nil
			}
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
	m.config.UseServerAddressAsSType = false
	if len(config) > 0 {
		err = json.Unmarshal(config, &m.config)
		if err != nil {
			cclog.ComponentError(m.name, "Error reading config:", err.Error())
			return err
		}
	}
	m.key = "mntpoint"
	if m.config.UseServerAddressAsSType {
		m.key = "server"
	}
	m.data = m.readNfsiostats()
	m.init = true
	return err
}

func (m *NfsIOStatCollector) Read(interval time.Duration, output chan lp.CCMetric) {
	timestamp := time.Now()

	// Get the current values for all mountpoints
	newdata := m.readNfsiostats()

	for mntpoint, values := range newdata {
		// Was the mount point already present in the last iteration
		if old, ok := m.data[mntpoint]; ok {
			// Calculate the difference of old and new values
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
				// Update old to the new value for the next iteration
				old[i] = values[i]
			}
		} else {
			// First time we see this mount point, store all values
			m.data[mntpoint] = values
		}
	}
	// Reset entries that do not exist anymore
	for mntpoint := range m.data {
		found := false
		for new := range newdata {
			if new == mntpoint {
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
	// Unset flag
	m.init = false
}
