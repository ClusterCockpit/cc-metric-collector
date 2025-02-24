package collectors

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	cclog "github.com/ClusterCockpit/cc-metric-collector/pkg/ccLogger"
	lp "github.com/ClusterCockpit/cc-lib/ccMessage"
)

// running average power limit (RAPL) monitoring attributes for a zone
type RAPLZoneInfo struct {
	// tags describing the RAPL zone:
	// * zone_name, subzone_name: e.g. psys, dram, core, uncore, package-0
	// * zone_id: e.g. 0:1 (zone 0 sub zone 1)
	tags            map[string]string
	energyFilepath  string    // path to a file containing the zones current energy counter in micro joules
	energy          int64     // current reading of the energy counter in micro joules
	energyTimestamp time.Time // timestamp when energy counter was read
	maxEnergyRange  int64     // Range of the above energy counter in micro-joules
}

type RAPLCollector struct {
	metricCollector
	config struct {
		// Exclude IDs for RAPL zones, e.g.
		// * 0 for zone 0
		// * 0:1 for zone 0 subzone 1
		ExcludeByID []string `json:"exclude_device_by_id,omitempty"`
		// Exclude names for RAPL zones, e.g. psys, dram, core, uncore, package-0
		ExcludeByName []string `json:"exclude_device_by_name,omitempty"`
	}
	RAPLZoneInfo []RAPLZoneInfo
	meta         map[string]string // default meta information
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
		name            string    // zones name e.g. psys, dram, core, uncore, package-0
		energyFilepath  string    // path to a file containing the zones current energy counter in micro joules
		energy          int64     // current reading of the energy counter in micro joules
		energyTimestamp time.Time // timestamp when energy counter was read
		maxEnergyRange  int64     // Range of the above energy counter in micro-joules
		ok              bool      // Are all information available?
	}) {
		// zones name e.g. psys, dram, core, uncore, package-0
		foundName := false
		if v, err :=
			os.ReadFile(
				filepath.Join(zonePath, "name")); err == nil {
			foundName = true
			z.name = strings.TrimSpace(string(v))
		}

		// path to a file containing the zones current energy counter in micro joules
		z.energyFilepath = filepath.Join(zonePath, "energy_uj")

		// current reading of the energy counter in micro joules
		foundEnergy := false
		if v, err := os.ReadFile(z.energyFilepath); err == nil {
			// timestamp when energy counter was read
			z.energyTimestamp = time.Now()
			if i, err := strconv.ParseInt(strings.TrimSpace(string(v)), 10, 64); err == nil {
				foundEnergy = true
				z.energy = i
			}
		}

		// Range of the above energy counter in micro-joules
		foundMaxEnergyRange := false
		if v, err :=
			os.ReadFile(
				filepath.Join(zonePath, "max_energy_range_uj")); err == nil {
			if i, err := strconv.ParseInt(strings.TrimSpace(string(v)), 10, 64); err == nil {
				foundMaxEnergyRange = true
				z.maxEnergyRange = i
			}
		}

		// Are all information available?
		z.ok = foundName && foundEnergy && foundMaxEnergyRange

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
		z := readZoneInfo(zonePath)
		if z.ok &&
			!isIDExcluded[zoneID] &&
			!isNameExcluded[z.name] {

			// Add RAPL monitoring attributes for a zone
			m.RAPLZoneInfo =
				append(
					m.RAPLZoneInfo,
					RAPLZoneInfo{
						tags: map[string]string{
							"id":        zoneID,
							"zone_name": z.name,
						},
						energyFilepath:  z.energyFilepath,
						energy:          z.energy,
						energyTimestamp: z.energyTimestamp,
						maxEnergyRange:  z.maxEnergyRange,
					})
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
				sz.ok &&
				!isIDExcluded[zoneID+":"+subZoneID] &&
				!isNameExcluded[sz.name] {
				m.RAPLZoneInfo =
					append(
						m.RAPLZoneInfo,
						RAPLZoneInfo{
							tags: map[string]string{
								"id":            zoneID + ":" + subZoneID,
								"zone_name":     z.name,
								"sub_zone_name": sz.name,
							},
							energyFilepath:  sz.energyFilepath,
							energy:          sz.energy,
							energyTimestamp: sz.energyTimestamp,
							maxEnergyRange:  sz.maxEnergyRange,
						})
			}
		}
	}

	if m.RAPLZoneInfo == nil {
		return fmt.Errorf("no running average power limit (RAPL) device found in %s", controlTypePath)

	}

	// Initialized
	cclog.ComponentDebug(
		m.name,
		"initialized",
		len(m.RAPLZoneInfo),
		"zones with running average power limit (RAPL) monitoring attributes")
	m.init = true

	return err
}

// Read reads running average power limit (RAPL) monitoring attributes for all initialized zones
// See: https://www.kernel.org/doc/html/latest/power/powercap/powercap.html#monitoring-attributes
func (m *RAPLCollector) Read(interval time.Duration, output chan lp.CCMessage) {

	for i := range m.RAPLZoneInfo {
		p := &m.RAPLZoneInfo[i]

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

				y, err := lp.NewMessage(
					"rapl_average_power",
					p.tags,
					m.meta,
					map[string]interface{}{"value": averagePower},
					energyTimestamp)
				if err == nil {
					output <- y
				}

				// Save current energy counter state
				p.energy = energy
				p.energyTimestamp = energyTimestamp
			}
		}
	}
}

// Close closes running average power limit (RAPL) metric collector
func (m *RAPLCollector) Close() {
	// Unset flag
	m.init = false
}
