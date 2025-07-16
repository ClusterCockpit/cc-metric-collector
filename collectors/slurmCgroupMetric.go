package collectors

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	cclog "github.com/ClusterCockpit/cc-lib/ccLogger"
	lp "github.com/ClusterCockpit/cc-lib/ccMessage"
)

type SlurmJobData struct {
	MemoryUsage      float64
	MaxMemoryUsage   float64
	LimitMemoryUsage float64
	CpuUsageUser     float64
	CpuUsageSys      float64
	CpuSet           []int
}

type SlurmCgroupsConfig struct {
	CgroupBase     string   `json:"cgroup_base"`
	ExcludeMetrics []string `json:"exclude_metrics,omitempty"`
}

type SlurmCgroupCollector struct {
	metricCollector
	config         SlurmCgroupsConfig
	meta           map[string]string
	tags           map[string]string
	allCPUs        []int
	cpuUsed        map[int]bool
	cgroupBase     string
	excludeMetrics map[string]struct{}
}

const defaultCgroupBase = "/sys/fs/cgroup/system.slice/slurmstepd.scope"

func ParseCPUs(cpuset string) ([]int, error) {
	var result []int
	if cpuset == "" {
		return result, nil
	}

	ranges := strings.Split(cpuset, ",")
	for _, r := range ranges {
		if strings.Contains(r, "-") {
			parts := strings.Split(r, "-")
			if len(parts) != 2 {
				return nil, fmt.Errorf("invalid CPU range: %s", r)
			}
			start, err := strconv.Atoi(strings.TrimSpace(parts[0]))
			if err != nil {
				return nil, fmt.Errorf("invalid CPU range start: %s", parts[0])
			}
			end, err := strconv.Atoi(strings.TrimSpace(parts[1]))
			if err != nil {
				return nil, fmt.Errorf("invalid CPU range end: %s", parts[1])
			}
			for i := start; i <= end; i++ {
				result = append(result, i)
			}
		} else {
			cpu, err := strconv.Atoi(strings.TrimSpace(r))
			if err != nil {
				return nil, fmt.Errorf("invalid CPU ID: %s", r)
			}
			result = append(result, cpu)
		}
	}
	return result, nil
}

func GetAllCPUs() ([]int, error) {
	data, err := os.ReadFile("/sys/devices/system/cpu/online")
	if err != nil {
		return nil, fmt.Errorf("failed to read /sys/devices/system/cpu/online: %v", err)
	}
	return ParseCPUs(strings.TrimSpace(string(data)))
}

func (m *SlurmCgroupCollector) isExcluded(metric string) bool {
	_, found := m.excludeMetrics[metric]
	return found
}

func (m *SlurmCgroupCollector) Init(config json.RawMessage) error {
	var err error
	m.name = "SlurmCgroupCollector"
	m.setup()
	m.parallel = true
	m.meta = map[string]string{"source": m.name, "group": "SLURM"}
	m.tags = map[string]string{"type": "hwthread"}
	m.cpuUsed = make(map[int]bool)

	m.cgroupBase = defaultCgroupBase

	if len(config) > 0 {
		err = json.Unmarshal(config, &m.config)
		m.excludeMetrics = make(map[string]struct{})
		for _, metric := range m.config.ExcludeMetrics {
			m.excludeMetrics[metric] = struct{}{}
		}
		if err != nil {
			cclog.ComponentError(m.name, "Error reading config:", err.Error())
			return err
		}
		if m.config.CgroupBase != "" {
			m.cgroupBase = m.config.CgroupBase
		}
	}

	m.allCPUs, err = GetAllCPUs()
	if err != nil {
		cclog.ComponentError(m.name, "Error reading online CPUs:", err.Error())
		return err
	}

	m.init = true
	return nil
}

func (m *SlurmCgroupCollector) ReadJobData(jobdir string) (SlurmJobData, error) {
	jobdata := SlurmJobData{
		MemoryUsage:      0,
		MaxMemoryUsage:   0,
		LimitMemoryUsage: 0,
		CpuUsageUser:     0,
		CpuUsageSys:      0,
		CpuSet:           []int{},
	}

	cg := func(f string) string { return filepath.Join(m.cgroupBase, jobdir, f) }

	memUsage, err := os.ReadFile(cg("memory.current"))
	if err == nil {
		x, err := strconv.ParseFloat(strings.TrimSpace(string(memUsage)), 64)
		if err == nil {
			jobdata.MemoryUsage = x
		}
	}

	maxMem, err := os.ReadFile(cg("memory.peak"))
	if err == nil {
		x, err := strconv.ParseFloat(strings.TrimSpace(string(maxMem)), 64)
		if err == nil {
			jobdata.MaxMemoryUsage = x
		}
	}

	limitMem, err := os.ReadFile(cg("memory.max"))
	if err == nil {
		x, err := strconv.ParseFloat(strings.TrimSpace(string(limitMem)), 64)
		if err == nil {
			jobdata.LimitMemoryUsage = x
		}
	}

	cpuStat, err := os.ReadFile(cg("cpu.stat"))
	if err == nil {
		lines := strings.Split(strings.TrimSpace(string(cpuStat)), "\n")
		var usageUsec, userUsec, systemUsec float64
		for _, line := range lines {
			fields := strings.Fields(line)
			if len(fields) < 2 {
				continue
			}
			value, err := strconv.ParseFloat(fields[1], 64)
			if err != nil {
				continue
			}
			switch fields[0] {
			case "usage_usec":
				usageUsec = value
			case "user_usec":
				userUsec = value
			case "system_usec":
				systemUsec = value
			}
		}
		if usageUsec > 0 {
			jobdata.CpuUsageUser = (userUsec * 100 / usageUsec)
			jobdata.CpuUsageSys = (systemUsec * 100 / usageUsec)
		}
	}

	cpuSet, err := os.ReadFile(cg("cpuset.cpus"))
	if err == nil {
		cpus, err := ParseCPUs(strings.TrimSpace(string(cpuSet)))
		if err == nil {
			jobdata.CpuSet = cpus
		}
	}

	return jobdata, nil
}

func (m *SlurmCgroupCollector) Read(interval time.Duration, output chan lp.CCMessage) {
	timestamp := time.Now()

	for k := range m.cpuUsed {
		delete(m.cpuUsed, k)
	}

	globPattern := filepath.Join(m.cgroupBase, "job_*")
	jobDirs, err := filepath.Glob(globPattern)
	if err != nil {
		cclog.ComponentError(m.name, "Error globbing job directories:", err.Error())
		return
	}

	for _, jdir := range jobDirs {
		jKey := filepath.Base(jdir)

		jobdata, err := m.ReadJobData(jKey)
		if err != nil {
			cclog.ComponentError(m.name, "Error reading job data for", jKey, ":", err.Error())
			continue
		}

		if len(jobdata.CpuSet) > 0 {
			coreCount := float64(len(jobdata.CpuSet))
			for _, cpu := range jobdata.CpuSet {
				coreTags := map[string]string{
					"type":    "hwthread",
					"type-id": fmt.Sprintf("%d", cpu),
				}

				if coreCount > 0 && !m.isExcluded("job_mem_used") {
					memPerCore := jobdata.MemoryUsage / coreCount
					if y, err := lp.NewMessage("job_mem_used", coreTags, m.meta, map[string]interface{}{"value": memPerCore}, timestamp); err == nil {
						y.AddMeta("unit", "Bytes")
						output <- y
					}
				}

				if coreCount > 0 && !m.isExcluded("job_max_mem_used") {
					maxMemPerCore := jobdata.MaxMemoryUsage / coreCount
					if y, err := lp.NewMessage("job_max_mem_used", coreTags, m.meta, map[string]interface{}{"value": maxMemPerCore}, timestamp); err == nil {
						y.AddMeta("unit", "Bytes")
						output <- y
					}
				}

				if coreCount > 0 && !m.isExcluded("job_mem_limit") {
					limitPerCore := jobdata.LimitMemoryUsage / coreCount
					if y, err := lp.NewMessage("job_mem_limit", coreTags, m.meta, map[string]interface{}{"value": limitPerCore}, timestamp); err == nil {
						y.AddMeta("unit", "Bytes")
						output <- y
					}
				}

				if coreCount > 0 && !m.isExcluded("job_user_cpu") {
					cpuUserPerCore := jobdata.CpuUsageUser / coreCount
					if y, err := lp.NewMessage("job_user_cpu", coreTags, m.meta, map[string]interface{}{"value": cpuUserPerCore}, timestamp); err == nil {
						y.AddMeta("unit", "%")
						output <- y
					}
				}

				if coreCount > 0 && !m.isExcluded("job_sys_cpu") {
					cpuSysPerCore := jobdata.CpuUsageSys / coreCount
					if y, err := lp.NewMessage("job_sys_cpu", coreTags, m.meta, map[string]interface{}{"value": cpuSysPerCore}, timestamp); err == nil {
						y.AddMeta("unit", "%")
						output <- y
					}
				}

				m.cpuUsed[cpu] = true
			}
		}
	}

	for _, cpu := range m.allCPUs {
		if !m.cpuUsed[cpu] {
			coreTags := map[string]string{
				"type":    "hwthread",
				"type-id": fmt.Sprintf("%d", cpu),
			}

			if !m.isExcluded("job_mem_used") {
				if y, err := lp.NewMessage("job_mem_used", coreTags, m.meta, map[string]interface{}{"value": 0}, timestamp); err == nil {
					y.AddMeta("unit", "Bytes")
					output <- y
				}
			}
			if !m.isExcluded("job_max_mem_used") {
				if y, err := lp.NewMessage("job_max_mem_used", coreTags, m.meta, map[string]interface{}{"value": 0}, timestamp); err == nil {
					y.AddMeta("unit", "Bytes")
					output <- y
				}
			}
			if !m.isExcluded("job_mem_limit") {
				if y, err := lp.NewMessage("job_mem_limit", coreTags, m.meta, map[string]interface{}{"value": 0}, timestamp); err == nil {
					y.AddMeta("unit", "Bytes")
					output <- y
				}
			}
			if !m.isExcluded("job_user_cpu") {
				if y, err := lp.NewMessage("job_user_cpu", coreTags, m.meta, map[string]interface{}{"value": 0}, timestamp); err == nil {
					y.AddMeta("unit", "%")
					output <- y
				}
			}
			if !m.isExcluded("job_sys_cpu") {
				if y, err := lp.NewMessage("job_sys_cpu", coreTags, m.meta, map[string]interface{}{"value": 0}, timestamp); err == nil {
					y.AddMeta("unit", "%")
					output <- y
				}
			}
		}
	}

}
func (m *SlurmCgroupCollector) Close() {
	m.init = false
}
