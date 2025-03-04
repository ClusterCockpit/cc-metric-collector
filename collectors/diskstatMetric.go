package collectors

import (
	"bufio"
	"encoding/json"
	"os"
	"strings"
	"syscall"
	"time"

	lp "github.com/ClusterCockpit/cc-lib/ccMessage"
	cclog "github.com/ClusterCockpit/cc-metric-collector/pkg/ccLogger"
)

const MOUNTFILE = `/proc/self/mounts`

type DiskstatCollectorConfig struct {
	ExcludeMetrics []string `json:"exclude_metrics,omitempty"`
	OnlyMetrics    []string `json:"only_metrics,omitempty"`
	ExcludeMounts  []string `json:"exclude_mounts,omitempty"`
}

type DiskstatCollector struct {
	metricCollector
	config DiskstatCollectorConfig
}

func (m *DiskstatCollector) shouldOutput(metricName string) bool {
	if len(m.config.OnlyMetrics) > 0 {
		for _, n := range m.config.OnlyMetrics {
			if n == metricName {
				return true
			}
		}
		return false
	}
	for _, n := range m.config.ExcludeMetrics {
		if n == metricName {
			return false
		}
	}
	return true
}

func (m *DiskstatCollector) Init(config json.RawMessage) error {
	m.name = "DiskstatCollector"
	m.parallel = true
	m.meta = map[string]string{"source": m.name, "group": "Disk"}
	m.setup()
	if len(config) > 0 {
		if err := json.Unmarshal(config, &m.config); err != nil {
			return err
		}
	}
	file, err := os.Open(MOUNTFILE)
	if err != nil {
		cclog.ComponentError(m.name, err.Error())
		return err
	}
	defer file.Close()
	m.init = true
	return nil
}

func (m *DiskstatCollector) Read(interval time.Duration, output chan lp.CCMessage) {
	if !m.init {
		return
	}

	file, err := os.Open(MOUNTFILE)
	if err != nil {
		cclog.ComponentError(m.name, err.Error())
		return
	}
	defer file.Close()

	part_max_used := uint64(0)
	scanner := bufio.NewScanner(file)
mountLoop:
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) == 0 {
			continue
		}
		if !strings.HasPrefix(line, "/dev") {
			continue
		}
		linefields := strings.Fields(line)
		if strings.Contains(linefields[0], "loop") || strings.Contains(linefields[1], "boot") {
			continue
		}

		mountPath := strings.Replace(linefields[1], `\040`, " ", -1)
		for _, excl := range m.config.ExcludeMounts {
			if strings.Contains(mountPath, excl) {
				continue mountLoop
			}
		}

		stat := syscall.Statfs_t{}
		if err := syscall.Statfs(mountPath, &stat); err != nil {
			continue
		}
		if stat.Blocks == 0 || stat.Bsize == 0 {
			continue
		}

		tags := map[string]string{"type": "node", "device": linefields[0]}
		total := (stat.Blocks * uint64(stat.Bsize)) / uint64(1000000000)
		if m.shouldOutput("disk_total") {
			y, err := lp.NewMessage("disk_total", tags, m.meta, map[string]interface{}{"value": total}, time.Now())
			if err == nil {
				y.AddMeta("unit", "GBytes")
				output <- y
			}
		}
		free := (stat.Bfree * uint64(stat.Bsize)) / uint64(1000000000)
		if m.shouldOutput("disk_free") {
			y, err := lp.NewMessage("disk_free", tags, m.meta, map[string]interface{}{"value": free}, time.Now())
			if err == nil {
				y.AddMeta("unit", "GBytes")
				output <- y
			}
		}
		if total > 0 {
			perc := (100 * (total - free)) / total
			if perc > part_max_used {
				part_max_used = perc
			}
		}
	}
	if m.shouldOutput("part_max_used") {
		y, err := lp.NewMessage("part_max_used", map[string]string{"type": "node"}, m.meta, map[string]interface{}{"value": int(part_max_used)}, time.Now())
		if err == nil {
			y.AddMeta("unit", "percent")
			output <- y
		}
	}
}

func (m *DiskstatCollector) Close() {
	m.init = false
}
