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

// These are the fields we read from the JSON configuration
type SlurmJobDetectorConfig struct {
	Interval        string `json:"interval"`
	SendJobEvents   bool   `json:"send_job_events,omitempty"`
	SendStepEvents  bool   `json:"send_step_events,omitempty"`
	SendJobMetrics  bool   `json:"send_job_metrics,omitempty"`
	SendStepMetrics bool   `json:"send_step_metrics,omitempty"`
	BaseDirectory   string `json:"sysfs_base,omitempty"`
	CgroupVersion   string `json:"cgroup_version"`
}

// This information is sent as JSON when an event occurs
type SlurmJobMetadata struct {
	UID             string   `json:"uid,omitempty"`
	JobId           string   `json:"jobid"`
	Timestamp       uint64   `json:"timestamp"`
	Status          string   `json:"status"`
	Step            string   `json:"step,omitempty"`
	Cpus            []int    `json:"cpus,omitempty"`
	Memories        []int    `json:"memories,omitempty"`
	MemoryLimitHard int64    `json:"memory_limit_hard,omitempty"`
	MemoryLimitSoft int64    `json:"memory_limit_soft,omitempty"`
	Devices         []string `json:"devices,omitempty"`
}

type SlurmJobMetrics struct {
	MemoryUsage      int64
	MaxMemoryUsage   int64
	LimitMemoryUsage int64
	CpuUsageUser     int64
	CpuUsageSys      int64
}

// This contains all variables we need during execution and the variables
// defined by metricCollector (name, init, ...)
type SlurmJobDetector struct {
	metricCollector
	config      SlurmJobDetectorConfig      // the configuration structure
	meta        map[string]string           // default meta information
	interval    time.Duration               // the interval parsed from configuration
	ticker      *time.Ticker                // own timer for event checking
	output      chan lp.CCMetric            // variable to save output channel at Read() for event sending
	wg          sync.WaitGroup              // sync group for event checking management
	done        chan bool                   // channel for event checking management
	directories map[string]SlurmJobMetadata // directory -> data mapping (data stored to re-send data in end job events)
}

const default_base_dir = "/sys/fs/cgroup"
const default_cgroup_version = "v1"

// not required to pre-initialized. Will be overwritten in Init() based on configuration
var cpuacct_base = filepath.Join(default_base_dir, "cpuacct", "slurm")
var memory_base = filepath.Join(default_base_dir, "memory", "slurm")
var cpuset_base = filepath.Join(default_base_dir, "cpuset", "slurm")
var devices_base = filepath.Join(default_base_dir, "devices", "slurm")

// Filenames for cgroup/v1
var limit_in_bytes_file = "memory.limit_in_bytes"
var soft_limit_in_bytes_file = "memory.soft_limit_in_bytes"
var cpus_effective_file = "cpuset.effective_cpus"
var mems_effective_file = "cpuset.effective_mems"
var devices_list_file = "devices.list"
var usage_in_bytes_file = "memory.usage_in_bytes"
var max_usage_in_bytes_file = "memory.max_usage_in_bytes"
var cpuacct_usage_file = "cpuacct.usage"
var cpuacct_usage_user_file = "cpuacct.usage_user"

// Filenames for cgroup/v2
// In Init() the filenames are set based on configuration
const soft_limit_in_bytes_file_v2 = "memory.high"
const limit_in_bytes_file_v2 = "memory.max"
const cpus_effective_file_v2 = "cpuset.cpus.effective"
const mems_effective_file_v2 = "cpuset.mems.effective"
const devices_list_file_v2 = "devices.list"
const usage_in_bytes_file_v2 = "memory.usage_in_bytes"
const max_usage_in_bytes_file_v2 = "memory.max_usage_in_bytes"
const cpuacct_usage_file_v2 = "cpuacct.usage"
const cpuacct_usage_user_file_v2 = "cpuacct.usage_user"

func fileToInt64(filename string) (int64, error) {
	data, err := os.ReadFile(filename)
	if err == nil {
		x, err := strconv.ParseInt(string(data), 0, 64)
		if err == nil {
			return x, err
		}
	}
	return 0, err
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
	// a *:* rwm
	return out
}

func GetPathParts(path string) []string {
	out := make([]string, 0)
	uid := ""
	jobid := ""
	step := ""
	parts := strings.Split(path, "/")
	// the folders of interest are at the end of the list, so traverse
	// from the back
	for i := len(parts) - 1; i >= 0; i-- {
		if strings.HasPrefix(parts[i], "uid_") {
			uid = parts[i]
		} else if strings.HasPrefix(parts[i], "job_") {
			jobid = parts[i]
		} else if strings.HasPrefix(parts[i], "step_") {
			step = parts[i]
		}
	}
	// only cgroup/v1 provides a uid but needs to be first entry
	if len(uid) > 0 {
		out = append(out, uid)
	}
	if len(jobid) > 0 {
		out = append(out, jobid)
	}
	// only if it's a step folder
	if len(step) > 0 {
		out = append(out, step)
	}
	return out
}

func GetIdsFromParts(parts []string) (string, string, string) {
	uid := ""
	jobid := ""
	step := ""

	for _, p := range parts {
		if strings.HasPrefix(p, "job_") {
			jobid = strings.TrimPrefix(p, "job_")
		} else if strings.HasPrefix(p, "uid_") {
			uid = strings.TrimPrefix(p, "uid_")
		} else if strings.HasPrefix(p, "step_") {
			step = strings.TrimPrefix(p, "step_")
		}
	}
	return uid, jobid, step
}

func (m *SlurmJobDetector) CheckEvents(timestamp time.Time) {
	var err error = nil
	var dirs []string = nil
	parts := make([]string, 3)
	parts = append(parts, cpuacct_base)
	if m.config.CgroupVersion == "v1" {
		parts = append(parts, "uid_[0-9]*")
	}
	parts = append(parts, "job_[0-9]*")

	dirs, err = filepath.Glob(filepath.Join(parts...))
	if err != nil {
		cclog.ComponentError(m.name, "Cannot get directory list for SLURM jobs")
		return
	}
	if m.config.SendStepEvents {
		parts = append(parts, "step_*")
		sdirs, err := filepath.Glob(filepath.Join(parts...))
		if err != nil {
			cclog.ComponentError(m.name, "Cannot get directory list for SLURM steps")
			return
		}
		dirs = append(dirs, sdirs...)
	}

	for _, d := range dirs {
		// Folder not in known directories map -> New job
		if _, ok := m.directories[d]; !ok {
			dirParts := GetPathParts(d)
			data, err := m.NewJobEvent(dirParts, timestamp, m.output)
			if err == nil {
				// Add the directory to the map
				cclog.ComponentDebug(m.name, "Adding directory ", d, " to known directories")
				m.directories[d] = data
			}
		}
	}
	for d, data := range m.directories {
		//  Known directory but it does not exist anymore -> Vanished/Finished job
		if _, ok := stringArrayContains(dirs, d); !ok {
			dirParts := GetPathParts(d)
			err := m.EndJobEvent(dirParts, data, timestamp, m.output)
			if err != nil {
				uid, jobid, step := GetIdsFromParts(dirParts)
				if len(step) == 0 {
					cclog.ComponentError(m.name, "Failed to end job for user ", uid, " jobid ", jobid)
				} else {
					cclog.ComponentError(m.name, "Failed to end job for user ", uid, " jobid ", jobid, " step ", step)
				}
			}
			// Remove the directory from the map
			cclog.ComponentDebug(m.name, "Removing directory ", d, " to known directories")
			delete(m.directories, d)
		}
	}
}

func (m *SlurmJobDetector) NewJobEvent(parts []string, timestamp time.Time, output chan lp.CCMetric) (SlurmJobMetadata, error) {
	uid, jobid, step := GetIdsFromParts(parts)
	pathstr := filepath.Join(parts...)
	if len(jobid) > 0 {
		cclog.ComponentError(m.name, "No jobid in path ", pathstr)
		return SlurmJobMetadata{}, fmt.Errorf("no jobid in path %s", pathstr)
	}
	jobtags := map[string]string{
		"type":    "job",
		"type-id": jobid,
	}

	// Fill job JSON with data from cgroup
	md := SlurmJobMetadata{
		JobId:     jobid,
		Timestamp: uint64(timestamp.Unix()),
		Status:    "start",
	}
	// cgroup/v2 has no uid in parts
	if len(uid) > 0 {
		md.UID = uid
	}
	if len(step) > 0 {
		md.Step = step
		jobtags["stype"] = "step"
		jobtags["stype-id"] = step
	}

	job_cpus, err := os.ReadFile(filepath.Join(cpuset_base, pathstr, cpus_effective_file))
	if err == nil {
		md.Cpus = ExpandList(string(job_cpus))
	}
	job_mems, err := os.ReadFile(filepath.Join(cpuset_base, pathstr, mems_effective_file))
	if err == nil {
		md.Memories = ExpandList(string(job_mems))
	}
	job_devs, err := os.ReadFile(filepath.Join(devices_base, pathstr, devices_list_file))
	if err == nil {
		md.Devices = ParseDevices(string(job_devs))
	}
	x, err := fileToInt64(filepath.Join(memory_base, pathstr, limit_in_bytes_file))
	if err == nil {
		md.MemoryLimitHard = x
	}
	x, err = fileToInt64(filepath.Join(memory_base, pathstr, soft_limit_in_bytes_file))
	if err == nil {
		md.MemoryLimitSoft = x
	}

	jobjson, err := json.Marshal(md)
	if err == nil {
		y, err := lp.New("slurm", jobtags, m.meta, map[string]interface{}{"value": string(jobjson)}, timestamp)
		if err == nil {
			if len(uid) > 0 {
				y.AddMeta("uid", uid)
				uname, err := osuser.LookupId(uid)
				if err == nil {
					y.AddMeta("username", uname.Username)
				}
			}
			y.AddMeta("metric_type", "event")
			output <- y
		}
	}
	return md, nil
}

// Not sure if it works with steps since the folders commonly do not vanish when a job step is finished
func (m *SlurmJobDetector) EndJobEvent(parts []string, data SlurmJobMetadata, timestamp time.Time, output chan lp.CCMetric) error {
	uid, jobid, step := GetIdsFromParts(parts)
	pathstr := filepath.Join(parts...)
	if len(jobid) > 0 {
		err := fmt.Errorf("no jobid in path %s", pathstr)
		cclog.ComponentError(m.name, err.Error())
		return err
	}
	jobtags := map[string]string{
		"type":    "job",
		"type-id": jobid,
	}

	// Fill job JSON with data from cgroup
	md := SlurmJobMetadata{
		JobId:           jobid,
		Timestamp:       uint64(timestamp.Unix()),
		Cpus:            data.Cpus,
		Memories:        data.Memories,
		Devices:         data.Devices,
		MemoryLimitHard: data.MemoryLimitHard,
		MemoryLimitSoft: data.MemoryLimitSoft,
		Status:          "end",
	}
	// cgroup/v2 has no uid in parts
	if len(uid) > 0 {
		md.UID = uid
	}
	if len(step) > 0 {
		md.Step = step
		jobtags["stype"] = "step"
		jobtags["stype-id"] = step
	}

	jobjson, err := json.Marshal(md)
	if err == nil {
		y, err := lp.New("slurm", jobtags, m.meta, map[string]interface{}{"value": string(jobjson)}, timestamp)
		if err == nil {
			if len(uid) > 0 {
				y.AddMeta("uid", uid)
				uname, err := osuser.LookupId(uid)
				if err == nil {
					y.AddMeta("username", uname.Username)
				}
			}
			y.AddMeta("metric_type", "event")
			output <- y
		} else {
			return err
		}
	} else {
		return err
	}
	return nil
}

func (m *SlurmJobDetector) ReadMetrics(parts []string) (SlurmJobMetrics, error) {
	jobdata := SlurmJobMetrics{
		MemoryUsage:      0,
		MaxMemoryUsage:   0,
		LimitMemoryUsage: 0,
		CpuUsageUser:     0,
		CpuUsageSys:      0,
	}

	part := filepath.Join(parts...)

	x, err := fileToInt64(filepath.Join(memory_base, part, usage_in_bytes_file))
	if err == nil {
		jobdata.MemoryUsage = x
	}
	x, err = fileToInt64(filepath.Join(memory_base, part, max_usage_in_bytes_file))
	if err == nil {
		jobdata.MaxMemoryUsage = x
	}
	tu, err := fileToInt64(filepath.Join(cpuacct_base, part, cpuacct_usage_file))
	if err == nil {
		uu, err := fileToInt64(filepath.Join(cpuacct_base, part, cpuacct_usage_user_file))
		if err == nil {
			jobdata.CpuUsageUser = int64(uu/tu) * 100
			jobdata.CpuUsageSys = 100 - jobdata.CpuUsageUser
		}

	}
	return jobdata, nil
}

func (m *SlurmJobDetector) SendMetrics(jobtags, jobmeta map[string]string, jobmetrics SlurmJobMetrics, timestamp time.Time, output chan lp.CCMetric) {

	y, err := lp.New("job_mem_used", jobtags, m.meta, map[string]interface{}{"value": jobmetrics.MemoryUsage}, timestamp)
	if err == nil {
		y.AddMeta("unit", "Bytes")
		for k, v := range jobmeta {
			y.AddMeta(k, v)
		}
		output <- y
	}
	y, err = lp.New("job_max_mem_used", jobtags, m.meta, map[string]interface{}{"value": jobmetrics.MaxMemoryUsage}, timestamp)
	if err == nil {
		y.AddMeta("unit", "Bytes")
		for k, v := range jobmeta {
			y.AddMeta(k, v)
		}
		output <- y
	}
	y, err = lp.New("job_cpu_user", jobtags, m.meta, map[string]interface{}{"value": jobmetrics.CpuUsageUser}, timestamp)
	if err == nil {
		y.AddMeta("unit", "%")
		for k, v := range jobmeta {
			y.AddMeta(k, v)
		}
		output <- y
	}
	y, err = lp.New("job_cpu_sys", jobtags, m.meta, map[string]interface{}{"value": jobmetrics.CpuUsageSys}, timestamp)
	if err == nil {
		y.AddMeta("unit", "%")
		for k, v := range jobmeta {
			y.AddMeta(k, v)
		}
		output <- y
	}
}

// Init initializes the sample collector
// Called once by the collector manager
// All tags, meta data tags and metrics that do not change over the runtime should be set here
func (m *SlurmJobDetector) Init(config json.RawMessage) error {
	var err error = nil
	m.name = "SlurmJobDetector"
	// This is for later use, also call it early
	m.setup()
	// Can be run in parallel with others
	m.parallel = true
	// Define meta information sent with each metric
	m.meta = map[string]string{"source": m.name, "group": "SLURM"}
	// Set configuration defaults
	m.config.SendJobEvents = false
	m.config.SendJobMetrics = false
	m.config.SendStepEvents = false
	m.config.SendStepMetrics = false
	m.config.CgroupVersion = default_cgroup_version
	m.config.BaseDirectory = default_base_dir
	// Read in the JSON configuration
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
	if m.config.CgroupVersion != "v1" && m.config.CgroupVersion != "v2" {
		cclog.ComponentError(m.name, "Invalid cgroup version", m.config.CgroupVersion, ":", err.Error())
		return err
	}

	// Storage for output channel
	m.output = nil
	// Management channel for the timer function.
	m.done = make(chan bool)
	// Create the own ticker
	m.ticker = time.NewTicker(m.interval)
	// Create space for storing files
	m.directories = make(map[string]SlurmJobMetadata)

	if _, err := os.Stat(m.config.BaseDirectory); err != nil {
		err := fmt.Errorf("cannot find base folder %s", m.config.BaseDirectory)
		cclog.ComponentError(m.name, err.Error())
		return err
	}
	cclog.ComponentDebug(m.name, "Using base directory", m.config.BaseDirectory)
	cpuacct_base = filepath.Join(m.config.BaseDirectory, "cpuacct", "slurm")
	memory_base = filepath.Join(m.config.BaseDirectory, "memory", "slurm")
	cpuset_base = filepath.Join(m.config.BaseDirectory, "cpuset", "slurm")
	devices_base = filepath.Join(m.config.BaseDirectory, "devices", "slurm")
	if m.config.CgroupVersion == "v2" {
		cclog.ComponentDebug(m.name, "Reconfiguring folders and filenames for cgroup/v2")
		cpuacct_base = filepath.Join(m.config.BaseDirectory, "system.slice", "slurmstepd.scope")
		memory_base = filepath.Join(m.config.BaseDirectory, "system.slice", "slurmstepd.scope")
		cpuset_base = filepath.Join(m.config.BaseDirectory, "system.slice", "slurmstepd.scope")
		devices_base = filepath.Join(m.config.BaseDirectory, "system.slice", "slurmstepd.scope")
		cpus_effective_file = cpus_effective_file_v2
		mems_effective_file = mems_effective_file_v2
		devices_list_file = devices_list_file_v2
		limit_in_bytes_file = limit_in_bytes_file_v2
		soft_limit_in_bytes_file = soft_limit_in_bytes_file_v2
		usage_in_bytes_file = usage_in_bytes_file_v2
		max_usage_in_bytes_file = max_usage_in_bytes_file_v2
		cpuacct_usage_file = cpuacct_usage_file_v2
		cpuacct_usage_user_file = cpuacct_usage_user_file_v2
	}
	if _, err := os.Stat(cpuacct_base); err != nil {
		err := fmt.Errorf("cannot find SLURM cgroup folder %s", cpuacct_base)
		cclog.ComponentError(m.name, err.Error())
		return err
	}

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
				if m.output != nil {
					cclog.ComponentDebug(m.name, "Checking events")
					m.CheckEvents(timestamp)
				}
			}
		}
	}()

	// Set this flag only if everything is initialized properly, all required files exist, ...
	m.init = true
	return err
}

// Read collects all metrics belonging to the SlurmJobDetector collector
// and sends them through the output channel to the collector manager
func (m *SlurmJobDetector) Read(interval time.Duration, output chan lp.CCMetric) {
	// Create a sample metric
	timestamp := time.Now()
	// Capture output channel for event sending in goroutine, so at startup, the event checking
	// waits until the first call of Read()
	m.output = output

	// This is the reading for metrics for all running jobs. For the event checking, check
	// the goroutine in Init()

	parts := make([]string, 0)
	parts = append(parts, cpuacct_base)
	// Only cgroup/v1 has a uid_* folder
	if m.config.CgroupVersion == "v1" {
		parts = append(parts, "uid_[0-9]*")
	}
	parts = append(parts, "job_[0-9]*")
	// Get folders based on constructed glob path
	dirs, err := filepath.Glob(filepath.Join(parts...))
	if err != nil {
		cclog.ComponentError(m.name, "Cannot get directory list for SLURM jobs")
		return
	}
	if m.config.SendStepEvents {
		// Add step lookup if we process step events
		parts = append(parts, "step_*")
		// Get step folders based on constructed glob path
		sdirs, err := filepath.Glob(filepath.Join(parts...))
		if err != nil {
			cclog.ComponentError(m.name, "Cannot get directory list for SLURM steps")
			return
		}
		// Add step folders to directory list for processsing
		dirs = append(dirs, sdirs...)
	}

	for _, d := range dirs {
		dirParts := GetPathParts(d)                   // Gets uid_*, job_* and step_* (if available)
		uid, jobid, step := GetIdsFromParts(dirParts) // extracts the IDs from the available parts

		// Create tags map for the job
		jobtags := map[string]string{
			"type":    "job",
			"type-id": jobid,
		}
		// Create meta map for the job
		jobmeta := make(map[string]string)

		// if cgroup/v1, we have a uid
		if len(uid) > 0 {
			jobmeta["uid"] = uid
			uname, err := osuser.LookupId(uid)
			if err == nil {
				jobmeta["username"] = uname.Username
			}
		}

		// if this is a step directory, add the sub type with value
		if len(step) > 0 {
			jobtags["stype"] = "step"
			jobtags["stype-id"] = step
		}
		jobmetrics, err := m.ReadMetrics(parts)
		if err != nil {
			// Send all metrics for the job
			m.SendMetrics(jobtags, jobmeta, jobmetrics, timestamp, output)
		}
	}
}

// Close metric collector: close network connection, close files, close libraries, ...
// Called once by the collector manager
func (m *SlurmJobDetector) Close() {
	m.done <- true
	m.wg.Wait()
	// Unset flag
	m.init = false
}
