package collectors

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"slices"
	"time"

	cclog "github.com/ClusterCockpit/cc-lib/v2/ccLogger"
	lp "github.com/ClusterCockpit/cc-lib/v2/ccMessage"
)

type SmartMonCollectorConfig struct {
	UseSudo        bool     `json:"use_sudo,omitempty"`
	ExcludeDevices []string `json:"exclude_devices,omitempty"`
	Devices        []struct {
		Name string `json:"name"`
		Type string `json:"type"`
	} `json:"devices,omitempty"`
}

type deviceT struct {
	Name         string `json:"name"`
	Type         string `json:"type"`
	queryCommand []string
}

type SmartMonCollector struct {
	metricCollector
	config      SmartMonCollectorConfig // the configuration structure
	meta        map[string]string       // default meta information
	tags        map[string]string       // default tags
	devices     []deviceT               // smartmon devices
	sudoCmd     string                  // Full path to 'sudo' command
	smartCtlCmd string                  // Full path to 'smartctl' command
}

func (m *SmartMonCollector) getSmartmonDevices() error {
	// Use configured devices
	if len(m.config.Devices) > 0 {
		for _, configDevice := range m.config.Devices {
			if !slices.Contains(m.config.ExcludeDevices, configDevice.Name) {
				d := deviceT{
					Name: configDevice.Name,
					Type: configDevice.Type,
				}
				if m.config.UseSudo {
					d.queryCommand = append(d.queryCommand, m.sudoCmd)
				}
				d.queryCommand = append(d.queryCommand, m.smartCtlCmd, "--json=c", "--device="+d.Type, "--all", d.Name)

				m.devices = append(m.devices, d)
			}
		}
		return nil
	}

	// Use scan command
	var scanCmd []string
	if m.config.UseSudo {
		scanCmd = append(scanCmd, m.sudoCmd)
	}
	scanCmd = append(scanCmd, m.smartCtlCmd, "--scan", "--json=c")
	command := exec.Command(scanCmd[0], scanCmd[1:]...)

	stdout, err := command.Output()
	if err != nil {
		return fmt.Errorf(
			"%s getSmartmonDevices(): Failed to execute device scan command %s: %w",
			m.name, command.String(), err)
	}

	var scanOutput struct {
		Devices []deviceT `json:"devices"`
	}
	err = json.Unmarshal(stdout, &scanOutput)
	if err != nil {
		return fmt.Errorf("%s getSmartmonDevices(): Failed to parse JSON output from device scan command: %w",
			m.name, err)
	}

	m.devices = make([]deviceT, 0)
	for _, d := range scanOutput.Devices {
		if !slices.Contains(m.config.ExcludeDevices, d.Name) {
			if m.config.UseSudo {
				d.queryCommand = append(d.queryCommand, m.sudoCmd)
			}
			d.queryCommand = append(d.queryCommand, m.smartCtlCmd, "--json=c", "--device="+d.Type, "--all", d.Name)

			m.devices = append(m.devices, d)
		}
	}

	return nil
}

func (m *SmartMonCollector) Init(config json.RawMessage) error {
	m.name = "SmartMonCollector"
	if err := m.setup(); err != nil {
		return fmt.Errorf("%s Init(): setup() call failed: %w", m.name, err)
	}
	m.parallel = true
	m.meta = map[string]string{
		"source": m.name,
		"group":  "Disk",
	}
	m.tags = map[string]string{
		"type":  "node",
		"stype": "disk",
	}

	// Read in the JSON configuration
	if len(config) > 0 {
		if err := json.Unmarshal(config, &m.config); err != nil {
			return fmt.Errorf("%s Init(): Error reading config: %w", m.name, err)
		}
	}

	// Check if sudo and smartctl are in search path
	if m.config.UseSudo {
		p, err := exec.LookPath("sudo")
		if err != nil {
			return fmt.Errorf("%s Init(): No sudo command found in search path: %w", m.name, err)
		}
		m.sudoCmd = p
	}
	p, err := exec.LookPath("smartctl")
	if err != nil {
		return fmt.Errorf("%s Init(): No smartctl command found in search path: %w", m.name, err)
	}
	m.smartCtlCmd = p

	if err = m.getSmartmonDevices(); err != nil {
		return err
	}

	m.init = true
	return err
}

type SmartMonData struct {
	SerialNumber string `json:"serial_number"`
	UserCapacity struct {
		Blocks int `json:"blocks"`
		Bytes  int `json:"bytes"`
	} `json:"user_capacity"`
	HealthLog struct {
		Temperature        int `json:"temperature"`
		PercentageUsed     int `json:"percentage_used"`
		AvailableSpare     int `json:"available_spare"`
		DataUnitsRead      int `json:"data_units_read"`
		DataUnitsWrite     int `json:"data_units_written"`
		HostReads          int `json:"host_reads"`
		HostWrites         int `json:"host_writes"`
		PowerCycles        int `json:"power_cycles"`
		PowerOnHours       int `json:"power_on_hours"`
		UnsafeShutdowns    int `json:"unsafe_shutdowns"`
		MediaErrors        int `json:"media_errors"`
		NumErrorLogEntries int `json:"num_err_log_entries"`
		WarnTempTime       int `json:"warning_temp_time"`
		CriticalTempTime   int `json:"critical_comp_time"`
	} `json:"nvme_smart_health_information_log"`
}

func (m *SmartMonCollector) Read(interval time.Duration, output chan lp.CCMessage) {
	timestamp := time.Now()
	for _, d := range m.devices {
		var data SmartMonData
		command := exec.Command(d.queryCommand[0], d.queryCommand[1:]...)

		stdout, err := command.Output()
		if err != nil {
			cclog.ComponentError(m.name, "cannot read data for device", d.Name)
			continue
		}
		err = json.Unmarshal(stdout, &data)
		if err != nil {
			cclog.ComponentError(m.name, "cannot unmarshal data for device", d.Name)
			continue
		}
		y, err := lp.NewMetric(
			"smartmon_temp", m.tags, m.meta, data.HealthLog.Temperature, timestamp)
		if err == nil {
			y.AddTag("stype-id", d.Name)
			y.AddMeta("unit", "degC")
			output <- y
		}
		y, err = lp.NewMetric(
			"smartmon_percent_used", m.tags, m.meta, data.HealthLog.PercentageUsed, timestamp)
		if err == nil {
			y.AddTag("stype-id", d.Name)
			y.AddMeta("unit", "percent")
			output <- y
		}
		y, err = lp.NewMetric(
			"smartmon_avail_spare", m.tags, m.meta, data.HealthLog.AvailableSpare, timestamp)
		if err == nil {
			y.AddTag("stype-id", d.Name)
			y.AddMeta("unit", "percent")
			output <- y
		}
		y, err = lp.NewMetric(
			"smartmon_data_units_read", m.tags, m.meta, data.HealthLog.DataUnitsRead, timestamp)
		if err == nil {
			y.AddTag("stype-id", d.Name)
			output <- y
		}
		y, err = lp.NewMetric(
			"smartmon_data_units_write", m.tags, m.meta, data.HealthLog.DataUnitsWrite, timestamp)
		if err == nil {
			y.AddTag("stype-id", d.Name)
			output <- y
		}
		y, err = lp.NewMetric(
			"smartmon_host_reads", m.tags, m.meta, data.HealthLog.HostReads, timestamp)
		if err == nil {
			y.AddTag("stype-id", d.Name)
			output <- y
		}
		y, err = lp.NewMetric(
			"smartmon_host_writes", m.tags, m.meta, data.HealthLog.HostWrites, timestamp)
		if err == nil {
			y.AddTag("stype-id", d.Name)
			output <- y
		}
		y, err = lp.NewMetric(
			"smartmon_power_cycles", m.tags, m.meta, data.HealthLog.PowerCycles, timestamp)
		if err == nil {
			y.AddTag("stype-id", d.Name)
			output <- y
		}
		y, err = lp.NewMetric(
			"smartmon_power_on", m.tags, m.meta, int64(data.HealthLog.PowerOnHours)*3600, timestamp)
		if err == nil {
			y.AddTag("stype-id", d.Name)
			y.AddMeta("unit", "sec")
			output <- y
		}
		y, err = lp.NewMetric(
			"smartmon_unsafe_shutdowns", m.tags, m.meta, data.HealthLog.UnsafeShutdowns, timestamp)
		if err == nil {
			y.AddTag("stype-id", d.Name)
			output <- y
		}
		y, err = lp.NewMetric(
			"smartmon_media_errors", m.tags, m.meta, data.HealthLog.MediaErrors, timestamp)
		if err == nil {
			y.AddTag("stype-id", d.Name)
			output <- y
		}
		y, err = lp.NewMetric(
			"smartmon_errlog_entries", m.tags, m.meta, data.HealthLog.NumErrorLogEntries, timestamp)
		if err == nil {
			y.AddTag("stype-id", d.Name)
			output <- y
		}
		y, err = lp.NewMetric(
			"smartmon_warn_temp_time", m.tags, m.meta, data.HealthLog.WarnTempTime, timestamp)
		if err == nil {
			y.AddTag("stype-id", d.Name)
			output <- y
		}
		y, err = lp.NewMetric(
			"smartmon_crit_temp_time", m.tags, m.meta, data.HealthLog.CriticalTempTime, timestamp)
		if err == nil {
			y.AddTag("stype-id", d.Name)
			output <- y
		}
	}
}

func (m *SmartMonCollector) Close() {
	m.init = false
}
