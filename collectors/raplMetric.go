package collectors

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	lp "github.com/ClusterCockpit/cc-energy-manager/pkg/cc-message"
	cclog "github.com/ClusterCockpit/cc-metric-collector/pkg/ccLogger"
)

// running average power limit (RAPL) monitoring attributes for a zone
// Only for Intel systems

type RAPLZoneInfo struct {
	energy            int64     // current reading of the energy counter in micro joules
	maxEnergyRange    int64     // Range of the above energy counter in micro-joules
	energyTimestamp   time.Time // timestamp when energy counter was read
	energyFilepath    string    // path to a file containing the zones current energy counter in micro joules
	shortTermFilepath string    // path to short term power limit
	longTermFilepath  string    // path to long term power limit
	enabledFilepath   string    // path to check whether limits are enabled
	name              string

	// tags describing the RAPL zone:
	// * zone_name, subzone_name: e.g. psys, dram, core, uncore, package-0
	// * zone_id: e.g. 0:1 (zone 0 sub zone 1)
	// type=socket for dram, core, uncore, package-* and type=node for psys
	// type-id=socket id
	tags map[string]string
}

type RAPLCollector struct {
	metricCollector
	config struct {
		// Exclude IDs for RAPL zones, e.g.
		// * 0 for zone 0
		// * 0:1 for zone 0 subzone 1
		ExcludeByID []string `json:"exclude_device_by_id,omitempty"`
		// Exclude names for RAPL zones, e.g. psys, dram, core, uncore, package-0
		ExcludeByName     []string `json:"exclude_device_by_name,omitempty"`
		SkipEnergyReading bool     `json:"skip_energy_reading,omitempty"`
		SkipLimitsReading bool     `json:"skip_limits_reading,omitempty"`
		OnlyEnabledLimits bool     `json:"only_enabled_limits,omitempty"`
	}
	raplZoneInfo []RAPLZoneInfo
	meta         map[string]string // default meta information
}

// Get the path to the power limit file for zone selectable by limit name
// Common limit names for Intel systems are
// - long_term
// - short_term
// Does not support AMD as AMD systems do not provide the power limits
// through sysfs
func ZoneLimitFile(folder string, limit_name string) string {
	nameGlob := filepath.Join(folder, "constraint_*_name")
	candidates, err := filepath.Glob(nameGlob)
	if err == nil {
		for _, c := range candidates {
			if v, err := os.ReadFile(c); err == nil {
				if strings.TrimSpace(string(v)) == limit_name {
					var i int
					n, err := fmt.Sscanf(filepath.Base(c), "constraint_%d_name", &i)
					if err == nil && n == 1 {
						return filepath.Join(folder, fmt.Sprintf("constraint_%d_power_limit_uw", i))
					}
				}
			}
		}
	}
	return ""
}

// Init initializes the running average power limit (RAPL) collector
func (m *RAPLCollector) Init(config json.RawMessage) error {

	// Check if already initialized
	if m.init {
		return nil
	}

	var err error = nil
	m.name = "RAPLCollector"
	m.setup()
	m.parallel = true
	m.meta = map[string]string{
		"source": m.name,
		"group":  "energy",
		"unit":   "Watt",
	}

	// Read in the JSON configuration
	m.config.SkipEnergyReading = false
	m.config.SkipLimitsReading = false
	m.config.OnlyEnabledLimits = true
	if len(config) > 0 {
		err = json.Unmarshal(config, &m.config)
		if err != nil {
			cclog.ComponentError(m.name, "Error reading config:", err.Error())
			return err
		}
	}

	// Configure excluded RAPL zones
	isIDExcluded := make(map[string]bool)
	if m.config.ExcludeByID != nil {
		for _, ID := range m.config.ExcludeByID {
			isIDExcluded[ID] = true
		}
	}
	isNameExcluded := make(map[string]bool)
	if m.config.ExcludeByName != nil {
		for _, name := range m.config.ExcludeByName {
			isNameExcluded[name] = true
		}
	}

	// readZoneInfo reads RAPL monitoring attributes for a zone given by zonePath
	// See: https://www.kernel.org/doc/html/latest/power/powercap/powercap.html#monitoring-attributes
	readZoneInfo := func(zonePath string) (z struct {
		name              string    // zones name e.g. psys, dram, core, uncore, package-0
		energyFilepath    string    // path to a file containing the zones current energy counter in micro joules
		energy            int64     // current reading of the energy counter in micro joules
		energyTimestamp   time.Time // timestamp when energy counter was read
		maxEnergyRange    int64     // Range of the above energy counter in micro-joules
		shortTermFilepath string
		longTermFilepath  string
		enabledFilepath   string
	}) {
		// zones name e.g. psys, dram, core, uncore, package-0

		if v, err :=
			os.ReadFile(
				filepath.Join(zonePath, "name")); err == nil {
			z.name = strings.TrimSpace(string(v))
		}

		if os.Geteuid() == 0 && (!m.config.SkipEnergyReading) {
			// path to a file containing the zones current energy counter in micro joules
			z.energyFilepath = filepath.Join(zonePath, "energy_uj")
			// current reading of the energy counter in micro joules
			if v, err := os.ReadFile(z.energyFilepath); err == nil {
				if i, err := strconv.ParseInt(strings.TrimSpace(string(v)), 10, 64); err == nil {
					z.energy = i
					// timestamp when energy counter was read
					z.energyTimestamp = time.Now()
				}
			} else {
				cclog.ComponentError(m.name, "Cannot read energy file for ", z.name, ":", err.Error())
			}
			// Range of the above energy counter in micro-joules
			if v, err :=
				os.ReadFile(
					filepath.Join(zonePath, "max_energy_range_uj")); err == nil {
				if i, err := strconv.ParseInt(strings.TrimSpace(string(v)), 10, 64); err == nil {
					z.maxEnergyRange = i
				}
			}
		} else {
			cclog.ComponentInfo(m.name, "Energy readings for", zonePath, "disabled")
		}

		if !m.config.SkipLimitsReading {
			z.shortTermFilepath = ZoneLimitFile(zonePath, "short_term")
			if _, err := os.Stat(z.shortTermFilepath); err != nil {
				z.shortTermFilepath = ""
			}
			z.longTermFilepath = ZoneLimitFile(zonePath, "long_term")
			if _, err := os.Stat(z.longTermFilepath); err != nil {
				z.longTermFilepath = ""
			}
			z.enabledFilepath = filepath.Join(zonePath, "enabled")
		} else {
			cclog.ComponentInfo(m.name, "Power limit readings for", zonePath, "disabled")
		}

		return
	}

	powerCapPrefix := "/sys/devices/virtual/powercap"
	controlType := "intel-rapl"
	controlTypePath := filepath.Join(powerCapPrefix, controlType)

	// Find all RAPL zones
	zonePrefix := filepath.Join(controlTypePath, controlType+":")
	zonesPath, err := filepath.Glob(zonePrefix + "*")
	if err != nil || zonesPath == nil {
		return fmt.Errorf("unable to find any zones under %s", controlTypePath)
	}

	for _, zonePath := range zonesPath {
		zoneID := strings.TrimPrefix(zonePath, zonePrefix)
		zonetags := make(map[string]string)

		z := readZoneInfo(zonePath)
		if !isIDExcluded[zoneID] &&
			!isNameExcluded[z.name] {

			si := RAPLZoneInfo{
				tags:              make(map[string]string),
				energyFilepath:    z.energyFilepath,
				energy:            z.energy,
				energyTimestamp:   z.energyTimestamp,
				maxEnergyRange:    z.maxEnergyRange,
				shortTermFilepath: z.shortTermFilepath,
				longTermFilepath:  z.longTermFilepath,
				enabledFilepath:   z.enabledFilepath,
				name:              z.name,
			}
			si.tags["type"] = "node"
			si.tags["type-id"] = "0"
			var pid int = 0
			if strings.HasPrefix(z.name, "package-") {
				n, err := fmt.Sscanf(z.name, "package-%d", &pid)
				if err == nil && n == 1 {
					si.tags["type-id"] = fmt.Sprintf("%d", pid)
					si.tags["type"] = "socket"
				}
				si.name = "pkg"
			}
			// Add RAPL monitoring attributes for a zone
			if _, ok1 := si.tags["type"]; ok1 {
				if _, ok2 := si.tags["type-id"]; ok2 {
					m.raplZoneInfo = append(m.raplZoneInfo, si)
					zonetags["type"] = si.tags["type"]
					zonetags["type-id"] = si.tags["type-id"]
				}
			}
		}

		// find all sub zones for the given zone
		subZonePrefix := filepath.Join(zonePath, controlType+":"+zoneID+":")
		subZonesPath, err := filepath.Glob(subZonePrefix + "*")
		if err != nil || subZonesPath == nil {
			continue
		}

		for _, subZonePath := range subZonesPath {
			subZoneID := strings.TrimPrefix(subZonePath, subZonePrefix)
			sz := readZoneInfo(subZonePath)

			if len(zoneID) > 0 && len(z.name) > 0 &&
				!isIDExcluded[zoneID+":"+subZoneID] &&
				!isNameExcluded[sz.name] {

				si := RAPLZoneInfo{
					tags:              zonetags,
					energyFilepath:    sz.energyFilepath,
					energy:            sz.energy,
					energyTimestamp:   sz.energyTimestamp,
					maxEnergyRange:    sz.maxEnergyRange,
					shortTermFilepath: sz.shortTermFilepath,
					longTermFilepath:  sz.longTermFilepath,
					enabledFilepath:   sz.enabledFilepath,
					name:              sz.name,
				}
				if _, ok1 := si.tags["type"]; ok1 {
					if _, ok2 := si.tags["type-id"]; ok2 {
						m.raplZoneInfo = append(m.raplZoneInfo, si)
					}
				}
			}
		}
	}

	if m.raplZoneInfo == nil {
		return fmt.Errorf("no running average power limit (RAPL) device found in %s", controlTypePath)

	}

	// Initialized
	cclog.ComponentDebug(
		m.name,
		"initialized",
		len(m.raplZoneInfo),
		"zones with running average power limit (RAPL) monitoring attributes")
	m.init = true

	return err
}

// Read reads running average power limit (RAPL) monitoring attributes for all initialized zones
// See: https://www.kernel.org/doc/html/latest/power/powercap/powercap.html#monitoring-attributes
func (m *RAPLCollector) Read(interval time.Duration, output chan lp.CCMessage) {

	for i := range m.raplZoneInfo {
		p := &m.raplZoneInfo[i]

		if os.Geteuid() == 0 && (!m.config.SkipEnergyReading) {
			// Read current value of the energy counter in micro joules
			if v, err := os.ReadFile(p.energyFilepath); err == nil {
				energyTimestamp := time.Now()
				if i, err := strconv.ParseInt(strings.TrimSpace(string(v)), 10, 64); err == nil {
					energy := i

					// Compute average power (Δ energy / Δ time)
					energyDiff := energy - p.energy
					if energyDiff < 0 {
						// Handle overflow:
						// ( p.maxEnergyRange - p.energy ) + energy
						// = p.maxEnergyRange + ( energy - p.energy )
						// = p.maxEnergyRange + diffEnergy
						energyDiff += p.maxEnergyRange
					}
					timeDiff := energyTimestamp.Sub(p.energyTimestamp)
					averagePower := float64(energyDiff) / float64(timeDiff.Microseconds())

					y, err := lp.NewMetric(
						fmt.Sprintf("rapl_%s_average_power", p.name),
						p.tags,
						m.meta,
						averagePower,
						energyTimestamp)
					if err == nil {
						output <- y
					}

					e, err := lp.NewMetric(
						fmt.Sprintf("rapl_%s_energy", p.name),
						p.tags,
						m.meta,
						float64(energyDiff)*1e-3,
						energyTimestamp)
					if err == nil {
						e.AddMeta("unit", "Joules")
						output <- e
					}

					// Save current energy counter state
					p.energy = energy
					p.energyTimestamp = energyTimestamp
				}
			}
		}
		// https://www.kernel.org/doc/html/latest/power/powercap/powercap.html#constraints
		if !m.config.SkipLimitsReading {
			skip := false
			if m.config.OnlyEnabledLimits {
				if v, err := os.ReadFile(p.enabledFilepath); err == nil {
					if strings.TrimSpace(string(v)) == "0" {
						skip = true
					}
				}
			}
			if !skip {
				if len(p.shortTermFilepath) > 0 {
					if v, err := os.ReadFile(p.shortTermFilepath); err == nil {
						if i, err := strconv.ParseInt(strings.TrimSpace(string(v)), 10, 64); err == nil {
							name := fmt.Sprintf("rapl_%s_limit_short_term", p.name)
							y, err := lp.NewMetric(name, p.tags, m.meta, i/1e6, time.Now())
							if err == nil {
								output <- y
							}
						}
					}
				}

				if len(p.longTermFilepath) > 0 {
					if v, err := os.ReadFile(p.longTermFilepath); err == nil {
						if i, err := strconv.ParseInt(strings.TrimSpace(string(v)), 10, 64); err == nil {
							name := fmt.Sprintf("rapl_%s_limit_long_term", p.name)
							y, err := lp.NewMetric(name, p.tags, m.meta, i/1e6, time.Now())
							if err == nil {
								output <- y
							}
						}
					}
				}
			}
		}
	}
}

// Close closes running average power limit (RAPL) metric collector
func (m *RAPLCollector) Close() {
	// Unset flag
	m.init = false
}
