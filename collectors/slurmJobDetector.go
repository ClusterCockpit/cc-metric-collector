package collectors

import (
	"encoding/json"
	"fmt"
	"os"
	osuser "os/user"
	filepath "path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	cclog "github.com/ClusterCockpit/cc-metric-collector/pkg/ccLogger"
	lp "github.com/ClusterCockpit/cc-metric-collector/pkg/ccMetric"
)

type SlurmJobMetadata struct {
	UID             uint64   `json:"uid"`
	JobId           uint64   `json:"jobid"`
	Timestamp       uint64   `json:"timestamp"`
	Status          string   `json:"status"`
	Step            string   `json:"step,omitempty"`
	Cpus            []int    `json:"cpus,omitempty"`
	Memories        []int    `json:"memories,omitempty"`
	MemoryLimitHard uint64   `json:"memory_limit_hard,omitempty"`
	MemoryLimitSoft uint64   `json:"memory_limit_soft,omitempty"`
	Devices         []string `json:"devices,omitempty"`
}

type SlurmJobMetrics struct {
	MemoryUsage      int64
	MaxMemoryUsage   int64
	LimitMemoryUsage int64
	CpuUsageUser     int64
	CpuUsageSys      int64
}

type SlurmJobStepData struct {
	Metrics SlurmJobMetrics
	Step    string
}

type SlurmJobData struct {
	Metrics SlurmJobMetrics
	Steps   []SlurmJobStepData
}

// These are the fields we read from the JSON configuration
type SlurmJobDetectorConfig struct {
	Interval        string   `json:"interval"`
	SendJobEvents   bool     `json:"send_job_events,omitempty"`
	SendStepEvents  bool     `json:"send_step_events,omitempty"`
	SendJobMetrics  bool     `json:"send_job_metrics,omitempty"`
	SendStepMetrics bool     `json:"send_step_metrics,omitempty"`
	ExcludeUsers    []string `json:"exclude_users,omitempty"`
	BaseDirectory   string   `json:"sysfs_base,omitempty"`
}

// This contains all variables we need during execution and the variables
// defined by metricCollector (name, init, ...)
type SlurmJobDetector struct {
	metricCollector
	config SlurmJobDetectorConfig // the configuration structure
	meta   map[string]string      // default meta information
	tags   map[string]string      // default tags
	//jobs     map[string]map[string]SlurmJobData
	interval time.Duration    // the interval parsed from configuration
	ticker   *time.Ticker     // own timer
	output   chan lp.CCMetric // own internal output channel
	wg       sync.WaitGroup   // sync group for management
	done     chan bool        // channel for management
	files    map[string]struct{}
}

const default_base_dir = "/sys/fs/cgroup"

var cpuacct_base = fmt.Sprintf("%s/cpuacct/slurm", default_base_dir)
var memory_base = fmt.Sprintf("%s/memory/slurm", default_base_dir)
var cpuset_base = fmt.Sprintf("%s/cpuset/slurm", default_base_dir)
var devices_base = fmt.Sprintf("%s/devices/slurm", default_base_dir)

func getSlurmJobs() []string {
	out := make([]string, 0)
	globpattern := filepath.Join(cpuacct_base, "uid_[0-9]*", "job_[0-9]*")

	dirs, err := filepath.Glob(globpattern)
	if err == nil {
		for _, d := range dirs {
			r, err := filepath.Rel(cpuacct_base, d)
			if err == nil {
				out = append(out, r)
			}
		}
	}
	return out
}

func getSlurmSteps() []string {
	out := make([]string, 0)
	globpattern := filepath.Join(cpuacct_base, "uid_[0-9]*", "job_[0-9]*", "step_*")

	dirs, err := filepath.Glob(globpattern)
	if err == nil {
		out = append(out, dirs...)
	}
	return out
}

func getId(prefix, str string) (uint64, error) {
	var s string
	format := prefix + "_%s"
	_, err := fmt.Sscanf(str, format, &s)
	if err != nil {
		return 0, err
	}
	id, err := strconv.ParseInt(s, 0, 64)
	if err != nil {
		return 0, err
	}
	return uint64(id), nil
}

func ExpandList(strlist string) []int {
	out := make([]int, 0)
	level1 := strings.Split(strlist, ",")
	if len(level1) > 0 {
		for _, entry := range level1 {
			var s, e int
			_, err := fmt.Sscanf(entry, "%d-%d", &s, &e)
			if err == nil {
				if s < e {
					for i := s; i <= e; i++ {
						out = append(out, i)
					}
				} else {
					for i := e; i <= s; i-- {
						out = append(out, i)
					}
				}
			} else {
				_, err := fmt.Sscanf(entry, "%d", &s)
				if err == nil {
					out = append(out, s)
				}
			}
		}
	}
	return out
}

func ParseDevices(devlist string) []string {
	out := make([]string, 0)
	return out
}

// Init initializes the sample collector
// Called once by the collector manager
// All tags, meta data tags and metrics that do not change over the runtime should be set here
func (m *SlurmJobDetector) Init(config json.RawMessage) error {
	var err error = nil
	// Always set the name early in Init() to use it in cclog.Component* functions
	m.name = "SlurmJobDetector"
	// This is for later use, also call it early
	m.setup()
	// Tell whether the collector should be run in parallel with others (reading files, ...)
	// or it should be run serially, mostly for collectors actually doing measurements
	// because they should not measure the execution of the other collectors
	m.parallel = true
	// Define meta information sent with each metric
	// (Can also be dynamic or this is the basic set with extension through AddMeta())
	m.meta = map[string]string{"source": m.name, "group": "SLURM"}
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
	m.config.SendJobEvents = false
	m.config.SendJobMetrics = false
	m.config.SendStepEvents = false
	m.config.SendStepMetrics = false
	m.config.BaseDirectory = default_base_dir
	if len(config) > 0 {
		err = json.Unmarshal(config, &m.config)
		if err != nil {
			cclog.ComponentError(m.name, "Error reading config:", err.Error())
			return err
		}
	}

	// Parse the read interval duration
	m.interval, err = time.ParseDuration(m.config.Interval)
	if err != nil {
		cclog.ComponentError(m.name, "Error parsing interval:", err.Error())
		return err
	}

	// Storage for output channel
	m.output = nil
	// Management channel for the timer function.
	m.done = make(chan bool)
	// Create the own ticker
	m.ticker = time.NewTicker(m.interval)
	// Create space for storing files
	m.files = make(map[string]struct{})

	cpuacct_base = fmt.Sprintf("%s/cpuacct/slurm", m.config.BaseDirectory)
	memory_base = fmt.Sprintf("%s/memory/slurm", m.config.BaseDirectory)
	cpuset_base = fmt.Sprintf("%s/cpuset/slurm", m.config.BaseDirectory)
	devices_base = fmt.Sprintf("%s/devices/slurm", m.config.BaseDirectory)
	cclog.ComponentDebug(m.name, "Using base directory", m.config.BaseDirectory)

	// Start the timer loop with return functionality by sending 'true' to the done channel
	m.wg.Add(1)
	go func() {
		for {
			select {
			case <-m.done:
				// Exit the timer loop
				cclog.ComponentDebug(m.name, "Closing...")
				m.wg.Done()
				return
			case timestamp := <-m.ticker.C:
				// This is executed every timer tick but we have to wait until the first
				// Read() to get the output channel
				cclog.ComponentDebug(m.name, "Checking events")
				if m.output != nil {
					m.CheckEvents(timestamp)
				}
			}
		}
	}()

	// Set this flag only if everything is initialized properly, all required files exist, ...
	m.init = true
	return err
}

func ReadJobData(userdir, jobdir string) (SlurmJobMetrics, error) {
	jobdata := SlurmJobMetrics{
		MemoryUsage:      0,
		MaxMemoryUsage:   0,
		LimitMemoryUsage: 0,
		CpuUsageUser:     0,
		CpuUsageSys:      0,
	}
	job_mem := filepath.Join(memory_base, userdir, jobdir, "memory.usage_in_bytes")
	mem_usage, err := os.ReadFile(job_mem)
	if err == nil {
		x, err := strconv.ParseInt(string(mem_usage), 0, 64)
		if err == nil {
			jobdata.MemoryUsage = x
		}
	}
	job_mem = filepath.Join(memory_base, userdir, jobdir, "memory.max_usage_in_bytes")
	mem_usage, err = os.ReadFile(job_mem)
	if err == nil {
		x, err := strconv.ParseInt(string(mem_usage), 0, 64)
		if err == nil {
			jobdata.MaxMemoryUsage = x
		}
	}
	job_cpu := filepath.Join(cpuacct_base, userdir, jobdir, "cpuacct.usage")
	total_usage, err := os.ReadFile(job_cpu)
	if err == nil {
		tu, err := strconv.ParseInt(string(total_usage), 0, 64)
		if err == nil {
			job_cpu = filepath.Join(cpuacct_base, userdir, jobdir, "cpuacct.usage_user")
			user_usage, err := os.ReadFile(job_cpu)
			if err == nil {
				uu, err := strconv.ParseInt(string(user_usage), 0, 64)
				if err == nil {
					jobdata.CpuUsageUser = int64(uu/tu) * 100
					jobdata.CpuUsageSys = 100 - jobdata.CpuUsageUser
				}
			}
		}
	}
	return jobdata, nil
}

func ReadJobStepData(userdir, jobdir, stepdir string) (SlurmJobMetrics, error) {
	jobdata := SlurmJobMetrics{
		MemoryUsage:      0,
		MaxMemoryUsage:   0,
		LimitMemoryUsage: 0,
		CpuUsageUser:     0,
		CpuUsageSys:      0,
	}
	job_mem := filepath.Join(memory_base, userdir, jobdir, stepdir, "memory.usage_in_bytes")
	mem_usage, err := os.ReadFile(job_mem)
	if err == nil {
		x, err := strconv.ParseInt(string(mem_usage), 0, 64)
		if err == nil {
			jobdata.MemoryUsage = x
		}
	}
	job_mem = filepath.Join(memory_base, userdir, jobdir, stepdir, "memory.max_usage_in_bytes")
	mem_usage, err = os.ReadFile(job_mem)
	if err == nil {
		x, err := strconv.ParseInt(string(mem_usage), 0, 64)
		if err == nil {
			jobdata.MaxMemoryUsage = x
		}
	}
	job_cpu := filepath.Join(cpuacct_base, userdir, jobdir, stepdir, "cpuacct.usage")
	total_usage, err := os.ReadFile(job_cpu)
	if err == nil {
		tu, err := strconv.ParseInt(string(total_usage), 0, 64)
		if err == nil {
			job_cpu = filepath.Join(cpuacct_base, userdir, jobdir, stepdir, "cpuacct.usage_user")
			user_usage, err := os.ReadFile(job_cpu)
			if err == nil {
				uu, err := strconv.ParseInt(string(user_usage), 0, 64)
				if err == nil {
					jobdata.CpuUsageUser = int64(uu/tu) * 100
					jobdata.CpuUsageSys = 100 - jobdata.CpuUsageUser
				}
			}
		}
	}
	return jobdata, nil
}

func pathinfo(path string) (uint64, uint64, string, error) {
	uid := uint64(0)
	jobid := uint64(0)
	step := ""

	parts := strings.Split(path, "/")
	for i := len(parts) - 1; i >= 0; i-- {
		p := parts[i]
		if strings.HasPrefix(p, "uid_") {
			u, err := getId("uid", p)
			if err == nil {
				uid = u
			}
		} else if strings.HasPrefix(p, "job_") {
			j, err := getId("job", p)
			if err == nil {
				jobid = j
			}
		} else if strings.HasPrefix(p, "step_") {
			step = p[5:]
		}
	}

	return uid, jobid, step, nil
}

func (m *SlurmJobDetector) CheckEvents(timestamp time.Time) {
	globPattern := filepath.Join(cpuacct_base, "uid_[0-9]*", "job_[0-9]*")
	if m.config.SendStepEvents {
		globPattern = filepath.Join(cpuacct_base, "uid_[0-9]*", "job_[0-9]*", "step_*")
	}
	dirs, err := filepath.Glob(globPattern)
	if err != nil {
		cclog.ComponentError(m.name, "Cannot glob with pattern", globPattern)
		return
	}
	for _, d := range dirs {
		if _, ok := m.files[d]; !ok {
			uid := uint64(0)
			jobid := uint64(0)
			step := ""
			uid, jobid, step, err = pathinfo(d)
			if err == nil {
				if len(step) == 0 {
					cclog.ComponentDebug(m.name, "New job for UID ", uid, " and JOBID ", jobid)
					m.NewJobEvent(uint64(uid), uint64(jobid), timestamp, m.output)
				} else {
					cclog.ComponentDebug(m.name, "New job step for UID ", uid, ", JOBID ", jobid, " and step ", step)
					m.NewJobStepEvent(uint64(uid), uint64(jobid), step, timestamp, m.output)
				}
			}
			m.files[d] = struct{}{}
		}
	}
	for d := range m.files {
		if _, ok := stringArrayContains(dirs, d); !ok {
			uid := uint64(0)
			jobid := uint64(0)
			step := ""
			uid, jobid, step, err = pathinfo(d)
			if err == nil {
				if len(step) == 0 {
					cclog.ComponentDebug(m.name, "Vanished job for UID ", uid, " and JOBID ", jobid)
					m.EndJobEvent(uint64(uid), uint64(jobid), timestamp, m.output)
				} else {
					cclog.ComponentDebug(m.name, "Vanished job step for UID ", uid, ", JOBID ", jobid, " and step ", step)
					m.EndJobStepEvent(uint64(uid), uint64(jobid), step, timestamp, m.output)
				}
			}
			delete(m.files, d)
		}
	}
}

func (m *SlurmJobDetector) NewJobEvent(uid, jobid uint64, timestamp time.Time, output chan lp.CCMetric) {
	jobtags := map[string]string{
		"type":    "job",
		"type-id": fmt.Sprintf("%d", jobid),
	}
	userdir := fmt.Sprintf("uid_%d", uid)
	jobdir := fmt.Sprintf("job_%d", uid)

	// Fill job JSON with data from cgroup
	var md SlurmJobMetadata
	job_cpus_file := filepath.Join(cpuset_base, userdir, jobdir, "cpuset.effective_cpus")
	job_cpus, err := os.ReadFile(job_cpus_file)
	if err == nil {
		md.Cpus = ExpandList(string(job_cpus))
	}
	job_mems_file := filepath.Join(cpuset_base, userdir, jobdir, "cpuset.effective_mems")
	job_mems, err := os.ReadFile(job_mems_file)
	if err == nil {
		md.Memories = ExpandList(string(job_mems))
	}
	job_devs_file := filepath.Join(devices_base, userdir, jobdir, "devices.list")
	job_devs, err := os.ReadFile(job_devs_file)
	if err == nil {
		md.Devices = ParseDevices(string(job_devs))
	}
	job_mem_limit_hard_file := filepath.Join(memory_base, userdir, jobdir, "memory.limit_in_bytes")
	job_mem_limit_hard, err := os.ReadFile(job_mem_limit_hard_file)
	if err == nil {
		x, err := strconv.ParseInt(string(job_mem_limit_hard), 0, 64)
		if err == nil {
			md.MemoryLimitHard = uint64(x)
		}
	}
	job_mem_limit_soft_file := filepath.Join(memory_base, userdir, jobdir, "memory.soft_limit_in_bytes")
	job_mem_limit_soft, err := os.ReadFile(job_mem_limit_soft_file)
	if err == nil {
		x, err := strconv.ParseInt(string(job_mem_limit_soft), 0, 64)
		if err == nil {
			md.MemoryLimitSoft = uint64(x)
		}
	}
	md.UID = uid
	md.JobId = jobid
	md.Timestamp = uint64(timestamp.Unix())
	md.Status = "start"
	jobjson, err := json.Marshal(md)
	if err == nil {
		y, err := lp.New("slurm", jobtags, m.meta, map[string]interface{}{"value": string(jobjson)}, timestamp)
		if err == nil {
			suid := fmt.Sprintf("%d", uid)
			y.AddMeta("uid", suid)
			uname, err := osuser.LookupId(suid)
			if err == nil {
				y.AddMeta("username", uname.Username)
			}
			y.AddMeta("metric_type", "event")
			output <- y
		}
	}
}

func (m *SlurmJobDetector) NewJobStepEvent(uid, jobid uint64, step string, timestamp time.Time, output chan lp.CCMetric) {
	jobtags := map[string]string{
		"type":     "job",
		"type-id":  fmt.Sprintf("%d", jobid),
		"stype":    "step",
		"stype-id": step,
	}
	userdir := fmt.Sprintf("uid_%d", uid)
	jobdir := fmt.Sprintf("job_%d", jobid)
	stepdir := fmt.Sprintf("step_%s", step)

	// Fill job JSON with data from cgroup
	var md SlurmJobMetadata
	job_cpus_file := filepath.Join(cpuset_base, userdir, jobdir, stepdir, "cpuset.effective_cpus")
	job_cpus, err := os.ReadFile(job_cpus_file)
	if err == nil {
		md.Cpus = ExpandList(string(job_cpus))
	}
	job_mems_file := filepath.Join(cpuset_base, userdir, jobdir, stepdir, "cpuset.effective_mems")
	job_mems, err := os.ReadFile(job_mems_file)
	if err == nil {
		md.Memories = ExpandList(string(job_mems))
	}
	job_devs_file := filepath.Join(devices_base, userdir, jobdir, stepdir, "devices.list")
	job_devs, err := os.ReadFile(job_devs_file)
	if err == nil {
		md.Devices = ParseDevices(string(job_devs))
	}
	job_mem_limit_hard_file := filepath.Join(memory_base, userdir, jobdir, stepdir, "memory.limit_in_bytes")
	job_mem_limit_hard, err := os.ReadFile(job_mem_limit_hard_file)
	if err == nil {
		x, err := strconv.ParseInt(string(job_mem_limit_hard), 0, 64)
		if err == nil {
			md.MemoryLimitHard = uint64(x)
		}
	}
	job_mem_limit_soft_file := filepath.Join(memory_base, userdir, jobdir, stepdir, "memory.soft_limit_in_bytes")
	job_mem_limit_soft, err := os.ReadFile(job_mem_limit_soft_file)
	if err == nil {
		x, err := strconv.ParseInt(string(job_mem_limit_soft), 0, 64)
		if err == nil {
			md.MemoryLimitSoft = uint64(x)
		}
	}
	md.UID = uid
	md.JobId = jobid
	md.Step = step
	md.Timestamp = uint64(timestamp.Unix())
	md.Status = "start"
	jobjson, err := json.Marshal(md)
	if err == nil {
		y, err := lp.New("slurm", jobtags, m.meta, map[string]interface{}{"value": string(jobjson)}, timestamp)
		if err == nil {
			suid := fmt.Sprintf("%d", uid)
			y.AddMeta("uid", suid)
			uname, err := osuser.LookupId(suid)
			if err == nil {
				y.AddMeta("username", uname.Username)
			}
			y.AddMeta("metric_type", "event")
			output <- y
		}
	}
}

func (m *SlurmJobDetector) EndJobEvent(uid, jobid uint64, timestamp time.Time, output chan lp.CCMetric) {
	jobtags := map[string]string{
		"type":    "job",
		"type-id": fmt.Sprintf("%d", jobid),
	}

	// Fill job JSON with data from cgroup
	var md SlurmJobMetadata
	md.UID = uid
	md.JobId = jobid
	md.Timestamp = uint64(timestamp.Unix())
	md.Status = "end"
	jobjson, err := json.Marshal(md)
	if err == nil {
		y, err := lp.New("slurm", jobtags, m.meta, map[string]interface{}{"value": string(jobjson)}, timestamp)
		if err == nil {
			suid := fmt.Sprintf("%d", uid)
			y.AddMeta("uid", suid)
			uname, err := osuser.LookupId(suid)
			if err == nil {
				y.AddMeta("username", uname.Username)
			}
			y.AddMeta("metric_type", "event")
			output <- y
		}
	}
}

func (m *SlurmJobDetector) EndJobStepEvent(uid, jobid uint64, step string, timestamp time.Time, output chan lp.CCMetric) {
	jobtags := map[string]string{
		"type":     "job",
		"type-id":  fmt.Sprintf("%d", jobid),
		"stype":    "step",
		"stype-id": step,
	}

	// Fill job JSON with data from cgroup
	var md SlurmJobMetadata
	md.UID = uid
	md.JobId = jobid
	md.Step = step
	md.Timestamp = uint64(timestamp.Unix())
	md.Status = "end"
	jobjson, err := json.Marshal(md)
	if err == nil {
		y, err := lp.New("slurm", jobtags, m.meta, map[string]interface{}{"value": string(jobjson)}, timestamp)
		if err == nil {
			suid := fmt.Sprintf("%d", uid)
			y.AddMeta("uid", suid)
			uname, err := osuser.LookupId(suid)
			if err == nil {
				y.AddMeta("username", uname.Username)
			}
			y.AddMeta("metric_type", "event")
			output <- y
		}
	}
}

func (m *SlurmJobDetector) SendMetrics(jobtags map[string]string, jobmetrics SlurmJobMetrics, timestamp time.Time, output chan lp.CCMetric) {

	y, err := lp.New("mem_used", jobtags, m.meta, map[string]interface{}{"value": jobmetrics.MemoryUsage}, timestamp)
	if err == nil {
		y.AddMeta("unit", "Bytes")
		output <- y
	}
	y, err = lp.New("max_mem_used", jobtags, m.meta, map[string]interface{}{"value": jobmetrics.MaxMemoryUsage}, timestamp)
	if err == nil {
		y.AddMeta("unit", "Bytes")
		output <- y
	}
	y, err = lp.New("user_cpu", jobtags, m.meta, map[string]interface{}{"value": jobmetrics.CpuUsageUser}, timestamp)
	if err == nil {
		y.AddMeta("unit", "%")
		output <- y
	}
	y, err = lp.New("user_sys", jobtags, m.meta, map[string]interface{}{"value": jobmetrics.CpuUsageSys}, timestamp)
	if err == nil {
		y.AddMeta("unit", "%")
		output <- y
	}
}

// Read collects all metrics belonging to the sample collector
// and sends them through the output channel to the collector manager
func (m *SlurmJobDetector) Read(interval time.Duration, output chan lp.CCMetric) {
	// Create a sample metric
	timestamp := time.Now()
	// Capture output channel
	m.output = output

	udirs, err := filepath.Glob(filepath.Join(cpuacct_base, "uid_[0-9]*"))
	if err != nil {
		return
	}

	for _, ud := range udirs {
		jdirs, err := filepath.Glob(filepath.Join(ud, "job_[0-9]*"))
		if err != nil {
			continue
		}
		uKey := filepath.Base(ud)

		for _, jd := range jdirs {
			jKey := filepath.Base(jd)
			jobid, err := getId("job", jKey)
			if err != nil {
				continue
			}
			jobmetrics, err := ReadJobData(uKey, jKey)
			if err != nil {
				jobtags := map[string]string{
					"type":    "job",
					"type-id": fmt.Sprintf("%d", jobid),
				}
				m.SendMetrics(jobtags, jobmetrics, timestamp, output)
			}
			if m.config.SendStepMetrics {
				sdirs, err := filepath.Glob(filepath.Join(jd, "step_*"))
				if err != nil {
					continue
				}
				for _, sd := range sdirs {
					sKey := filepath.Base(sd)
					stepmetrics, err := ReadJobStepData(uKey, jKey, sKey)
					if err != nil {
						continue
					}
					var stepname string
					_, err = fmt.Sscanf(sKey, "step_%s", &stepname)
					if err == nil {
						jobtags := map[string]string{
							"type":     "job",
							"type-id":  fmt.Sprintf("%d", jobid),
							"stype":    "step",
							"stype-id": stepname,
						}
						m.SendMetrics(jobtags, stepmetrics, timestamp, output)
					}
				}

			}
		}
	}

	// uid_pattern := "uid_[0-9]*"
	// job_pattern := "job_[0-9]*"
	// //step_pattern := "step_*"

	// globPattern := filepath.Join(cpuacct_base, uid_pattern)
	// uidDirs, err := filepath.Glob(globPattern)
	// if err != nil {
	// 	return
	// }
	// for _, udir := range uidDirs {
	// 	uKey := filepath.Base(udir)
	// 	if _, ok := m.jobs[uKey]; !ok {
	// 		m.jobs[uKey] = make(map[string]SlurmJobData)
	// 	}
	// 	uid, _ := getId("uid", uKey)
	// 	globPattern = filepath.Join(cpuacct_base, uKey, job_pattern)
	// 	jobDirs, err := filepath.Glob(globPattern)
	// 	if err != nil {
	// 		continue
	// 	}
	// 	for _, jdir := range jobDirs {
	// 		jKey := filepath.Base(jdir)
	// 		jobid, _ := getId("job", jKey)
	// 		if _, ok := m.jobs[uKey][jKey]; !ok {
	// 			var steps []SlurmJobStepData = nil
	// 			if m.config.SendStepEvents || m.config.SendStepMetrics {
	// 				steps = make([]SlurmJobStepData, 0)
	// 			}
	// 			m.jobs[uKey][jKey] = SlurmJobData{
	// 				Metrics: SlurmJobMetrics{
	// 					MemoryUsage:      0,
	// 					MaxMemoryUsage:   0,
	// 					LimitMemoryUsage: 0,
	// 					CpuUsageUser:     0,
	// 					CpuUsageSys:      0,
	// 				},
	// 				Steps: steps,
	// 			}
	// 			m.NewJobEvent(uid, jobid, timestamp, output)
	// 		}
	// 		jdata := m.jobs[uKey][jKey]

	// 		jobmetrics, err := ReadJobData(uKey, jKey)
	// 		if err == nil {

	// 			jdata.Metrics = jobmetrics

	// 			m.SendMetrics(jobid, jobmetrics, timestamp, output)
	// 		}
	// 		m.jobs[uKey][jKey] = jdata
	// 	}
	// }

	// for uKey, udata := range m.jobs {
	// 	uid, _ := getId("uid", uKey)
	// 	for jKey := range udata {
	// 		jobid, _ := getId("job", jKey)
	// 		p := filepath.Join(cpuset_base, uKey, jKey)
	// 		if _, err := os.Stat(p); err != nil {
	// 			m.EndJobEvent(uid, jobid, timestamp, output)
	// 			delete(udata, jKey)
	// 		}
	// 	}
	// 	p := filepath.Join(cpuset_base, uKey)
	// 	if _, err := os.Stat(p); err != nil {
	// 		delete(udata, uKey)
	// 	}
	// }

}

// Close metric collector: close network connection, close files, close libraries, ...
// Called once by the collector manager
func (m *SlurmJobDetector) Close() {
	m.done <- true
	m.wg.Wait()
	// Unset flag
	m.init = false
}
