package collectors

import (
	"encoding/json"
	"os/exec"
	"time"

	cclog "github.com/ClusterCockpit/cc-metric-collector/internal/ccLogger"
	lp "github.com/ClusterCockpit/cc-metric-collector/internal/ccMetric"
)

type SmartMonCollectorConfig struct {
	UseSudo        bool     `json:"use_sudo"`
	ExcludeDevices []string `json:"exclude_devices"`
}

type SmartMonCollector struct {
	metricCollector
	config      SmartMonCollectorConfig // the configuration structure
	meta        map[string]string       // default meta information
	tags        map[string]string       // default tags
	devices     []string                // smartmon devices
	sudoCmd     string                  // Full path to 'sudo' command
	smartCtlCmd string                  // Full path to 'smartctl' command
}

func (m *SmartMonCollector) getSmartmonDevices() error {
	var command *exec.Cmd
	var scan struct {
		Devices []struct {
			Name string `json:"name"`
			Type string `json:"type"`
		} `json:"devices"`
	}
	m.devices = make([]string, 0)
	if m.config.UseSudo {
		command = exec.Command(m.sudoCmd, m.smartCtlCmd, "--scan", "-j")
	} else {
		command = exec.Command(m.smartCtlCmd, "--scan", "-j")
	}
	command.Wait()
	stdout, err := command.Output()
	if err != nil {
		return err
	}
	err = json.Unmarshal(stdout, &scan)
	if err != nil {
		return err
	}
	for _, d := range scan.Devices {
		if len(d.Name) > 0 {
			m.devices = append(m.devices, d.Name)
		}
	}

	return nil
}

func (m *SmartMonCollector) Init(config json.RawMessage) error {
	var err error = nil
	m.name = "SmartMonCollector"
	m.setup()
	m.parallel = true
	m.meta = map[string]string{"source": m.name, "group": "Disk"}
	m.tags = map[string]string{"type": "node", "stype": "disk"}
	// Read in the JSON configuration
	if len(config) > 0 {
		err = json.Unmarshal(config, &m.config)
		if err != nil {
			cclog.ComponentError(m.name, "Error reading config:", err.Error())
			return err
		}
	}
	if m.config.UseSudo {
		p, err := exec.LookPath("sudo")
		if err != nil {
			return err
		}
		m.sudoCmd = p
	}
	p, err := exec.LookPath("smartctl")
	if err != nil {
		return err
	}
	m.smartCtlCmd = p
	err = m.getSmartmonDevices()
	if err != nil {
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

func (m *SmartMonCollector) Read(interval time.Duration, output chan lp.CCMetric) {
	timestamp := time.Now()
	for _, d := range m.devices {
		var command *exec.Cmd
		var data SmartMonData
		if m.config.UseSudo {
			command = exec.Command(m.sudoCmd, m.smartCtlCmd, "-j", "-a", d)
		} else {
			command = exec.Command(m.smartCtlCmd, "-j", "-a", d)
		}
		command.Wait()
		stdout, err := command.Output()
		if err != nil {
			cclog.ComponentError(m.name, "cannot read data for device", d)
			continue
		}
		err = json.Unmarshal(stdout, &data)
		if err != nil {
			cclog.ComponentError(m.name, "cannot unmarshal data for device", d)
			continue
		}
		y, err := lp.New("smartmon_temp", m.tags, m.meta, map[string]interface{}{"value": data.HealthLog.Temperature}, timestamp)
		if err == nil {
			y.AddTag("stype-id", d)
			y.AddMeta("unit", "degC")
			output <- y
		}
		y, err = lp.New("smartmon_percent_used", m.tags, m.meta, map[string]interface{}{"value": data.HealthLog.PercentageUsed}, timestamp)
		if err == nil {
			y.AddTag("stype-id", d)
			y.AddMeta("unit", "percent")
			output <- y
		}
		y, err = lp.New("smartmon_avail_spare", m.tags, m.meta, map[string]interface{}{"value": data.HealthLog.AvailableSpare}, timestamp)
		if err == nil {
			y.AddTag("stype-id", d)
			y.AddMeta("unit", "percent")
			output <- y
		}
		y, err = lp.New("smartmon_data_units_read", m.tags, m.meta, map[string]interface{}{"value": data.HealthLog.DataUnitsRead}, timestamp)
		if err == nil {
			y.AddTag("stype-id", d)
			output <- y
		}
		y, err = lp.New("smartmon_data_units_write", m.tags, m.meta, map[string]interface{}{"value": data.HealthLog.DataUnitsWrite}, timestamp)
		if err == nil {
			y.AddTag("stype-id", d)
			output <- y
		}
		y, err = lp.New("smartmon_host_reads", m.tags, m.meta, map[string]interface{}{"value": data.HealthLog.HostReads}, timestamp)
		if err == nil {
			y.AddTag("stype-id", d)
			output <- y
		}
		y, err = lp.New("smartmon_host_writes", m.tags, m.meta, map[string]interface{}{"value": data.HealthLog.HostWrites}, timestamp)
		if err == nil {
			y.AddTag("stype-id", d)
			output <- y
		}
		y, err = lp.New("smartmon_power_cycles", m.tags, m.meta, map[string]interface{}{"value": data.HealthLog.PowerCycles}, timestamp)
		if err == nil {
			y.AddTag("stype-id", d)
			output <- y
		}
		y, err = lp.New("smartmon_power_on", m.tags, m.meta, map[string]interface{}{"value": int64(data.HealthLog.PowerOnHours) * 3600}, timestamp)
		if err == nil {
			y.AddTag("stype-id", d)
			y.AddMeta("unit", "seconds")
			output <- y
		}
		y, err = lp.New("smartmon_unsafe_shutdowns", m.tags, m.meta, map[string]interface{}{"value": data.HealthLog.UnsafeShutdowns}, timestamp)
		if err == nil {
			y.AddTag("stype-id", d)
			output <- y
		}
		y, err = lp.New("smartmon_media_errors", m.tags, m.meta, map[string]interface{}{"value": data.HealthLog.MediaErrors}, timestamp)
		if err == nil {
			y.AddTag("stype-id", d)
			output <- y
		}
		y, err = lp.New("smartmon_errlog_entries", m.tags, m.meta, map[string]interface{}{"value": data.HealthLog.NumErrorLogEntries}, timestamp)
		if err == nil {
			y.AddTag("stype-id", d)
			output <- y
		}
		y, err = lp.New("smartmon_warn_temp_time", m.tags, m.meta, map[string]interface{}{"value": data.HealthLog.WarnTempTime}, timestamp)
		if err == nil {
			y.AddTag("stype-id", d)
			output <- y
		}
		y, err = lp.New("smartmon_crit_temp_time", m.tags, m.meta, map[string]interface{}{"value": data.HealthLog.CriticalTempTime}, timestamp)
		if err == nil {
			y.AddTag("stype-id", d)
			output <- y
		}
	}

}

func (m *SmartMonCollector) Close() {
	m.init = false
}
