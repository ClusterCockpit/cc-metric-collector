package collectors

import (
	"encoding/json"
	"errors"
	"os/exec"
	"regexp"
	"strings"
	"time"

	cclog "github.com/ClusterCockpit/cc-metric-collector/pkg/ccLogger"
	lp "github.com/ClusterCockpit/cc-metric-collector/pkg/ccMetric"
)

type LustreJobstatCollectorConfig struct {
	LCtlCommand        string   `json:"lctl_command,omitempty"`
	ExcludeMetrics     []string `json:"exclude_metrics,omitempty"`
	Sudo               bool     `json:"use_sudo,omitempty"`
	SendAbsoluteValues bool     `json:"send_abs_values,omitempty"`
	SendDerivedValues  bool     `json:"send_derived_values,omitempty"`
	SendDiffValues     bool     `json:"send_diff_values,omitempty"`
	JobRegex           string   `json:"jobid_regex,omitempty"`
}

type LustreJobstatCollector struct {
	metricCollector
	tags          map[string]string
	config        LustreJobstatCollectorConfig
	lctl          string
	sudoCmd       string
	lastTimestamp time.Time                // Store time stamp of last tick to derive bandwidths
	definitions   []LustreMetricDefinition // Combined list without excluded metrics
	//stats             map[string]map[string]int64 // Data for last value per device and metric
	lastMdtData       *map[string]map[string]LustreMetricData
	lastObdfilterData *map[string]map[string]LustreMetricData
	jobidRegex        *regexp.Regexp
}

var defaultJobidRegex = `^(?P<jobid>[\d\w\.]+)$`

var LustreMetricJobstatsDefinition = []LustreMetricDefinition{
	{
		name:       "lustre_job_read_samples",
		lineprefix: "read",
		offsetname: "samples",
		unit:       "requests",
		calc:       "none",
	},
	{
		name:       "lustre_job_read_min_bytes",
		lineprefix: "read_bytes",
		offsetname: "min",
		unit:       "bytes",
		calc:       "none",
	},
	{
		name:       "lustre_job_read_max_bytes",
		lineprefix: "read_bytes",
		offsetname: "max",
		unit:       "bytes",
		calc:       "none",
	},
}

func (m *LustreJobstatCollector) executeLustreCommand(option string) []string {
	return executeLustreCommand(m.sudoCmd, m.lctl, LCTL_OPTION, option, m.config.Sudo)
}

func (m *LustreJobstatCollector) Init(config json.RawMessage) error {
	var err error
	m.name = "LustreJobstatCollector"
	m.parallel = true
	m.config.JobRegex = defaultJobidRegex
	m.config.SendAbsoluteValues = true
	if len(config) > 0 {
		err = json.Unmarshal(config, &m.config)
		if err != nil {
			return err
		}
	}
	m.setup()
	m.tags = map[string]string{"type": "jobid"}
	m.meta = map[string]string{"source": m.name, "group": "Lustre", "scope": "job"}

	// Lustre file system statistics can only be queried by user root
	// or with password-less sudo
	// if !m.config.Sudo {
	// 	user, err := user.Current()
	// 	if err != nil {
	// 		cclog.ComponentError(m.name, "Failed to get current user:", err.Error())
	// 		return err
	// 	}
	// 	if user.Uid != "0" {
	// 		cclog.ComponentError(m.name, "Lustre file system statistics can only be queried by user root")
	// 		return err
	// 	}
	// } else {
	// 	p, err := exec.LookPath("sudo")
	// 	if err != nil {
	// 		cclog.ComponentError(m.name, "Cannot find 'sudo'")
	// 		return err
	// 	}
	// 	m.sudoCmd = p
	// }

	p, err := exec.LookPath(m.config.LCtlCommand)
	if err != nil {
		p, err = exec.LookPath(LCTL_CMD)
		if err != nil {
			return err
		}
	}
	m.lctl = p

	m.definitions = make([]LustreMetricDefinition, 0)
	if m.config.SendAbsoluteValues {
		for _, def := range LustreMetricJobstatsDefinition {
			if _, skip := stringArrayContains(m.config.ExcludeMetrics, def.name); !skip {
				m.definitions = append(m.definitions, def)
			}
		}
	}

	if len(m.definitions) == 0 {
		return errors.New("no metrics to collect")
	}

	x := make(map[string]map[string]LustreMetricData)
	m.lastMdtData = &x
	x = make(map[string]map[string]LustreMetricData)
	m.lastObdfilterData = &x

	if len(m.config.JobRegex) > 0 {
		jregex := strings.ReplaceAll(m.config.JobRegex, "%", "\\")
		r, err := regexp.Compile(jregex)
		if err == nil {
			m.jobidRegex = r
		} else {
			cclog.ComponentError(m.name, "Cannot compile jobid regex")
			return err
		}
	}

	m.lastTimestamp = time.Now()
	m.init = true
	return nil
}

func (m *LustreJobstatCollector) Read(interval time.Duration, output chan lp.CCMetric) {
	if !m.init {
		return
	}

	getValue := func(data map[string]map[string]LustreMetricData, device string, jobid string, operation string, field string) int64 {
		var value int64 = -1
		if ddata, ok := data[device]; ok {
			if jdata, ok := ddata[jobid]; ok {
				if opdata, ok := jdata.op_data[operation]; ok {
					if v, ok := opdata[field]; ok {
						value = v
					}
				}
			}
		}
		return value
	}

	jobIdToTags := func(jobregex *regexp.Regexp, job string) map[string]string {
		tags := make(map[string]string)
		groups := jobregex.SubexpNames()
		for _, match := range jobregex.FindAllStringSubmatch(job, -1) {
			for groupIdx, group := range match {
				if len(groups[groupIdx]) > 0 {
					tags[groups[groupIdx]] = group
				}
			}
		}
		return tags
	}

	generateMetric := func(definition LustreMetricDefinition, data map[string]map[string]LustreMetricData, last map[string]map[string]LustreMetricData, now time.Time) {
		tdiff := now.Sub(m.lastTimestamp)
		for dev, ddata := range data {
			for jobid, jdata := range ddata {
				jobtags := jobIdToTags(m.jobidRegex, jobid)
				if _, ok := jobtags["jobid"]; !ok {
					continue
				}
				cur := getValue(data, dev, jobid, definition.lineprefix, definition.offsetname)
				old := getValue(last, dev, jobid, definition.lineprefix, definition.offsetname)
				var x interface{} = -1
				var valid = false
				switch definition.calc {
				case "none":
					x = cur
					valid = true
				case "difference":
					if len(last) > 0 {
						if old >= 0 {
							x = cur - old
							valid = true
						}
					}
				case "derivative":
					if len(last) > 0 {
						if old >= 0 {
							x = float64(cur-old) / tdiff.Seconds()
							valid = true
						}
					}
				}
				if valid {
					y, err := lp.New(definition.name, m.tags, m.meta, map[string]interface{}{"value": x}, now)
					if err == nil {
						y.AddTag("stype", "device")
						y.AddTag("stype-id", dev)
						if j, ok := jobtags["jobid"]; ok {
							y.AddTag("type-id", j)
						} else {
							y.AddTag("type-id", jobid)
						}
						for k, v := range jobtags {
							switch k {
							case "jobid":
							case "hostname":
								y.AddTag("hostname", v)
							default:
								y.AddMeta(k, v)
							}
						}
						if len(definition.unit) > 0 {
							y.AddMeta("unit", definition.unit)
						} else {
							if unit, ok := jdata.op_units[definition.lineprefix]; ok {
								y.AddMeta("unit", unit)
							}
						}
						output <- y
					}
				}
			}
		}
	}

	now := time.Now()

	mdt_lines := m.executeLustreCommand("mdt.*.job_stats")
	if len(mdt_lines) > 0 {
		mdt_data := readCommandOutput(mdt_lines)
		for _, def := range m.definitions {
			generateMetric(def, mdt_data, *m.lastMdtData, now)
		}
		m.lastMdtData = &mdt_data
	}

	obdfilter_lines := m.executeLustreCommand("obdfilter.*.job_stats")
	if len(obdfilter_lines) > 0 {
		obdfilter_data := readCommandOutput(obdfilter_lines)
		for _, def := range m.definitions {
			generateMetric(def, obdfilter_data, *m.lastObdfilterData, now)
		}
		m.lastObdfilterData = &obdfilter_data
	}

	m.lastTimestamp = now
}

func (m *LustreJobstatCollector) Close() {
	m.init = false
}
