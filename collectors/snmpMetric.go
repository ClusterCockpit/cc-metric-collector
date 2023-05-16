package collectors

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	cclog "github.com/ClusterCockpit/cc-metric-collector/pkg/ccLogger"
	lp "github.com/ClusterCockpit/cc-metric-collector/pkg/ccMetric"
	"github.com/gosnmp/gosnmp"
)

type SNMPCollectorTargetConfig struct {
	Hostname  string `json:"hostname"`
	Port      int    `json:"port,omitempty"`
	Community string `json:"community"`
	Timeout   int    `json:"timeout"` // timeout in seconds
}

type SNMPCollectorMetricConfig struct {
	Name  string `json:"name"`
	Value string `json:"value"`
	Unit  string `json:"unit,omitempty"`
}

// These are the fields we read from the JSON configuration
type SNMPCollectorConfig struct {
	Targets []SNMPCollectorTargetConfig `json:"targets"`
	Metrics []SNMPCollectorMetricConfig `json:"metrics"`
}

// This contains all variables we need during execution and the variables
// defined by metricCollector (name, init, ...)
type SNMPCollector struct {
	metricCollector
	config SNMPCollectorConfig // the configuration structure
	meta   map[string]string   // default meta information
	tags   map[string]string   // default tags
}

func validOid(oid string) bool {
	match, err := regexp.MatchString(`^[012]\.(?:[0-9]|[1-3][0-9])(\.\d+)*$`, oid)
	if err != nil {
		return false
	}
	return match
}

// Init initializes the snmp collector
// Called once by the collector manager
// All tags, meta data tags and metrics that do not change over the runtime should be set here
func (m *SNMPCollector) Init(config json.RawMessage) error {
	var err error = nil
	// Always set the name early in Init() to use it in cclog.Component* functions
	m.name = "SNMPCollector"
	// This is for later use, also call it early
	m.setup()
	// Tell whether the collector should be run in parallel with others (reading files, ...)
	// or it should be run serially, mostly for collectors actually doing measurements
	// because they should not measure the execution of the other collectors
	m.parallel = true
	// Define meta information sent with each metric
	// (Can also be dynamic or this is the basic set with extension through AddMeta())
	m.meta = map[string]string{"source": m.name, "group": "SNMP"}
	// Define tags sent with each metric
	// The 'type' tag is always needed, it defines the granularity of the metric
	// node -> whole system
	// socket -> CPU socket (requires socket ID as 'type-id' tag)
	// die -> CPU die (requires CPU die ID as 'type-id' tag)
	// memoryDomain -> NUMA domain (requires NUMA domain ID as 'type-id' tag)
	// llc -> Last level cache (requires last level cache ID as 'type-id' tag)
	// core -> single CPU core that may consist of multiple hardware threads (SMT) (requires core ID as 'type-id' tag)
	// hwthtread -> single CPU hardware thread (requires hardware thread ID as 'type-id' tag)
	// accelerator -> A accelerator device like GPU or FPGA (requires an accelerator ID as 'type-id' tag)
	m.tags = map[string]string{"type": "node"}
	// Read in the JSON configuration
	if len(config) > 0 {
		err = json.Unmarshal(config, &m.config)
		if err != nil {
			cclog.ComponentError(m.name, "Error reading config:", err.Error())
			return err
		}
	}

	if len(m.config.Targets) == 0 {
		err = fmt.Errorf("no targets configured, exiting")
		cclog.ComponentError(m.name, err.Error())
		return err
	}

	if len(m.config.Metrics) == 0 {
		err = fmt.Errorf("no metrics configured, exiting")
		cclog.ComponentError(m.name, err.Error())
		return err
	}

	// Set this flag only if everything is initialized properly, all required files exist, ...
	m.init = true
	return err
}

// Read collects all metrics belonging to the snmp collector
// and sends them through the output channel to the collector manager
func (m *SNMPCollector) Read(interval time.Duration, output chan lp.CCMetric) {
	// Create a snmp metric
	timestamp := time.Now()

	for _, target := range m.config.Targets {
		port := uint16(161)
		if target.Port > 0 {
			port = uint16(target.Port)
		}
		comm := "public"
		if len(target.Community) > 0 {
			comm = target.Community
		}
		timeout := 1
		if target.Timeout > 0 {
			timeout = target.Timeout
		}
		params := &gosnmp.GoSNMP{
			Target:    target.Hostname,
			Port:      port,
			Community: comm,
			Version:   gosnmp.Version2c,
			Timeout:   time.Duration(timeout) * time.Second,
		}
		err := params.Connect()
		if err != nil {
			cclog.ComponentError(m.name, err.Error())
			continue
		}
		for _, metric := range m.config.Metrics {
			if !validOid(metric.Value) {
				continue
			}
			oids := []string{}
			name := gosnmp.SnmpPDU{
				Value: metric.Name,
				Name:  metric.Name,
			}
			nameidx := -1
			value := gosnmp.SnmpPDU{
				Value: 0,
				Name:  metric.Value,
			}
			valueidx := -1
			unit := gosnmp.SnmpPDU{
				Value: metric.Unit,
				Name:  metric.Unit,
			}
			unitidx := -1
			if validOid(metric.Name) {
				oids = append(oids, metric.Name)
				nameidx = 0
			}
			if validOid(metric.Value) {
				oids = append(oids, metric.Value)
				valueidx = 1
			}
			if len(metric.Unit) > 0 && validOid(metric.Unit) {
				oids = append(oids, metric.Unit)
				unitidx = 2
			}
			result, err := gosnmp.Default.Get(oids)
			if err != nil {
				cclog.ComponentError(m.name, "failed to get data for OIDs %s", strings.Join(oids, ","))
				continue
			}
			if nameidx >= 0 && len(result.Variables) > nameidx {
				name = result.Variables[nameidx]
			}
			if valueidx >= 0 && len(result.Variables) > valueidx {
				value = result.Variables[valueidx]
			}
			if unitidx >= 0 && len(result.Variables) > unitidx {
				unit = result.Variables[unitidx]
			}
			if len(result.Variables) > 2 {
				unit = result.Variables[2]
			}
			y, err := lp.New(name.Value.(string), m.tags, m.meta, map[string]interface{}{"value": value.Value}, timestamp)
			if err == nil {
				// Send it to output channel
				if len(unit.Name) > 0 && unit.Value != nil {
					y.AddMeta("unit", unit.Value.(string))
				}
				output <- y
			}
		}
		params.Conn.Close()
	}
}

// Close metric collector: close network connection, close files, close libraries, ...
// Called once by the collector manager
func (m *SNMPCollector) Close() {
	// Unset flag
	m.init = false
}
