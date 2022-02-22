package collectors

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"

	cclog "github.com/ClusterCockpit/cc-metric-collector/internal/ccLogger"
	lp "github.com/ClusterCockpit/cc-metric-collector/internal/ccMetric"
)

//	"log"

const MOUNTFILE = `/proc/self/mounts`

type DiskstatCollectorConfig struct {
	ExcludeMetrics []string `json:"exclude_metrics,omitempty"`
}

type DiskstatCollector struct {
	metricCollector
	//matches map[string]int
	config IOstatCollectorConfig
	//devices map[string]IOstatCollectorEntry
}

func (m *DiskstatCollector) Init(config json.RawMessage) error {
	m.name = "DiskstatCollector"
	m.meta = map[string]string{"source": m.name, "group": "Disk"}
	m.setup()
	if len(config) > 0 {
		err := json.Unmarshal(config, &m.config)
		if err != nil {
			return err
		}
	}
	file, err := os.Open(string(MOUNTFILE))
	if err != nil {
		cclog.ComponentError(m.name, err.Error())
		return err
	}
	defer file.Close()
	m.init = true
	return nil
}

func (m *DiskstatCollector) Read(interval time.Duration, output chan lp.CCMetric) {
	if !m.init {
		return
	}

	file, err := os.Open(string(MOUNTFILE))
	if err != nil {
		cclog.ComponentError(m.name, err.Error())
		return
	}
	defer file.Close()

	part_max_used := uint64(0)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) == 0 {
			continue
		}
		if !strings.HasPrefix(line, "/dev") {
			continue
		}
		linefields := strings.Fields(line)
		if strings.Contains(linefields[0], "loop") {
			continue
		}
		if strings.Contains(linefields[1], "boot") {
			continue
		}
		path := strings.Replace(linefields[1], `\040`, " ", -1)
		stat := syscall.Statfs_t{}
		err := syscall.Statfs(path, &stat)
		if err != nil {
			fmt.Println(err.Error())
			return
		}
		tags := map[string]string{"type": "node", "device": linefields[0]}
		total := (stat.Blocks * uint64(stat.Bsize)) / uint64(1000000000)
		y, err := lp.New("disk_total", tags, m.meta, map[string]interface{}{"value": total}, time.Now())
		if err == nil {
			y.AddMeta("unit", "GBytes")
			output <- y
		}
		free := (stat.Bfree * uint64(stat.Bsize)) / uint64(1000000000)
		y, err = lp.New("disk_free", tags, m.meta, map[string]interface{}{"value": free}, time.Now())
		if err == nil {
			y.AddMeta("unit", "GBytes")
			output <- y
		}
		perc := (100 * (total - free)) / total
		if perc > part_max_used {
			part_max_used = perc
		}
	}
	y, err := lp.New("part_max_used", map[string]string{"type": "node"}, m.meta, map[string]interface{}{"value": int(part_max_used)}, time.Now())
	if err == nil {
		y.AddMeta("unit", "percent")
		output <- y
	}
}

func (m *DiskstatCollector) Close() {
	m.init = false
}
