package receivers

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	cclog "github.com/ClusterCockpit/cc-metric-collector/pkg/ccLogger"
	lp "github.com/ClusterCockpit/cc-metric-collector/pkg/ccMetric"
	"github.com/gosnmp/gosnmp"
)

type SNMPReceiverTargetConfig struct {
	Hostname  string `json:"hostname"`
	Port      int    `json:"port,omitempty"`
	Community string `json:"community,omitempty"`
	Timeout   string `json:"timeout,omitempty"`
	timeout   time.Duration
	Version   string `json:"version,omitempty"`
	Type      string `json:"type,omitempty"`
	TypeId    string `json:"type-id,omitempty"`
	SubType   string `json:"subtype,omitempty"`
	SubTypeId string `json:"subtype-id,omitempty"`
}

type SNMPReceiverMetricConfig struct {
	Name string `json:"name"`
	OID  string `json:"oid"`
	Unit string `json:"unit,omitempty"`
}

// SNMPReceiver configuration: receiver type, listen address, port
type SNMPReceiverConfig struct {
	Type         string                     `json:"type"`
	Targets      []SNMPReceiverTargetConfig `json:"targets"`
	Metrics      []SNMPReceiverMetricConfig `json:"metrics"`
	ReadInterval string                     `json:"read_interval,omitempty"`
}

type SNMPReceiver struct {
	receiver
	config SNMPReceiverConfig

	// Storage for static information
	meta map[string]string
	tags map[string]string
	// Use in case of own go routine
	done     chan bool
	wg       sync.WaitGroup
	interval time.Duration
}

func validOid(oid string) bool {
	// Regex from https://github.com/BornToBeRoot/NETworkManager/blob/6805740762bf19b95051c7eaa73cf2b4727733c3/Source/NETworkManager.Utilities/RegexHelper.cs#L88
	// Match on leading dot added by Thomas Gruber <thomas.gruber@fau.de>
	match, err := regexp.MatchString(`^[\.]?[012]\.(?:[0-9]|[1-3][0-9])(\.\d+)*$`, oid)
	if err != nil {
		return false
	}
	return match
}

func (r *SNMPReceiver) readTarget(target SNMPReceiverTargetConfig, output chan lp.CCMetric) {
	port := uint16(161)
	comm := "public"
	timeout := time.Duration(1) * time.Second
	version := gosnmp.Version2c
	timestamp := time.Now()
	if target.Port > 0 {
		port = uint16(target.Port)
	}
	if len(target.Community) > 0 {
		comm = target.Community
	}
	if target.timeout > 0 {
		timeout = target.timeout
	}
	if len(target.Version) > 0 {
		switch target.Version {
		case "1":
			version = gosnmp.Version1
		case "2c":
			version = gosnmp.Version2c
		case "3":
			version = gosnmp.Version3
		default:
			cclog.ComponentError(r.name, "Invalid SNMP version ", target.Version)
			return
		}
	}
	params := &gosnmp.GoSNMP{
		Target:    target.Hostname,
		Port:      port,
		Community: comm,
		Version:   version,
		Timeout:   timeout,
	}
	err := params.Connect()
	if err != nil {
		cclog.ComponentError(r.name, err.Error())
		return
	}
	for _, metric := range r.config.Metrics {
		if !validOid(metric.OID) {
			cclog.ComponentDebug(r.name, "Skipping ", metric.Name, ", not valid OID: ", metric.OID)
			continue
		}
		oids := make([]string, 0)
		name := gosnmp.SnmpPDU{
			Value: metric.Name,
			Name:  metric.Name,
		}
		nameidx := -1
		value := gosnmp.SnmpPDU{
			Value: nil,
			Name:  metric.OID,
		}
		valueidx := -1
		unit := gosnmp.SnmpPDU{
			Value: metric.Unit,
			Name:  metric.Unit,
		}
		unitidx := -1
		idx := 0
		if validOid(metric.Name) {
			oids = append(oids, metric.Name)
			nameidx = idx
			idx = idx + 1
		}
		if validOid(metric.OID) {
			oids = append(oids, metric.OID)
			valueidx = idx
			idx = idx + 1
		}
		if len(metric.Unit) > 0 && validOid(metric.Unit) {
			oids = append(oids, metric.Unit)
			unitidx = idx
		}
		//cclog.ComponentDebug(r.name, len(oids), oids)
		result, err := params.Get(oids)
		if err != nil {
			cclog.ComponentError(r.name, "failed to get data for OIDs ", strings.Join(oids, ","), ": ", err.Error())
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
		tags := r.tags
		if len(target.Type) > 0 {
			tags["type"] = target.Type
		}
		if len(target.TypeId) > 0 {
			tags["type-id"] = target.TypeId
		}
		if len(target.SubType) > 0 {
			tags["stype"] = target.SubType
		}
		if len(target.SubTypeId) > 0 {
			tags["stype-id"] = target.SubTypeId
		}
		if value.Value != nil {
			y, err := lp.New(name.Value.(string), tags, r.meta, map[string]interface{}{"value": value.Value}, timestamp)
			if err == nil {
				if len(unit.Name) > 0 && unit.Value != nil {
					y.AddMeta("unit", unit.Value.(string))
				}
				output <- y
			}
		}
	}
	params.Conn.Close()
}

// Implement functions required for Receiver interface
// Start(), Close()
// See: metricReceiver.go

func (r *SNMPReceiver) Start() {
	cclog.ComponentDebug(r.name, "START")

	r.done = make(chan bool)
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()

		// Create ticker
		ticker := time.NewTicker(r.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// process ticker event -> continue
				if r.sink != nil {
					for _, t := range r.config.Targets {
						select {
						case <-r.done:
							return
						default:
							r.readTarget(t, r.sink)
						}
					}
				}
				continue
			case <-r.done:
				return
			}
		}
	}()
}

// Close receiver: close network connection, close files, close libraries, ...
func (r *SNMPReceiver) Close() {
	cclog.ComponentDebug(r.name, "CLOSE")

	r.done <- true
	r.wg.Wait()
}

// New function to create a new instance of the receiver
// Initialize the receiver by giving it a name and reading in the config JSON
func NewSNMPReceiver(name string, config json.RawMessage) (Receiver, error) {
	var err error = nil
	r := new(SNMPReceiver)

	// Set name of SNMPReceiver
	// The name should be chosen in such a way that different instances of SNMPReceiver can be distinguished
	r.name = fmt.Sprintf("SNMPReceiver(%s)", name)

	// Set static information
	r.meta = map[string]string{"source": r.name, "group": "SNMP"}
	r.tags = map[string]string{"type": "node"}

	// Set defaults in r.config
	r.interval = time.Duration(30) * time.Second

	// Read the sample receiver specific JSON config
	if len(config) > 0 {
		err := json.Unmarshal(config, &r.config)
		if err != nil {
			cclog.ComponentError(r.name, "Error reading config:", err.Error())
			return nil, err
		}
	}

	// Check that all required fields in the configuration are set
	if len(r.config.Targets) == 0 {
		err = fmt.Errorf("no targets configured, exiting")
		cclog.ComponentError(r.name, err.Error())
		return nil, err
	}

	if len(r.config.Metrics) == 0 {
		err = fmt.Errorf("no metrics configured, exiting")
		cclog.ComponentError(r.name, err.Error())
		return nil, err
	}

	if len(r.config.ReadInterval) > 0 {
		d, err := time.ParseDuration(r.config.ReadInterval)
		if err != nil {
			err = fmt.Errorf("failed to parse read interval, exiting")
			cclog.ComponentError(r.name, err.Error())
			return nil, err
		}
		r.interval = d
	}
	newtargets := make([]SNMPReceiverTargetConfig, 0)
	for _, t := range r.config.Targets {
		t.timeout = time.Duration(1) * time.Second
		if len(t.Timeout) > 0 {
			d, err := time.ParseDuration(t.Timeout)
			if err != nil {
				err = fmt.Errorf("failed to parse interval for target %s", t.Hostname)
				cclog.ComponentError(r.name, err.Error())
				continue
			}
			t.timeout = d
		}
		newtargets = append(newtargets, t)
	}
	r.config.Targets = newtargets

	return r, nil
}
