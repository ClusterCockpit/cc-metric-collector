package collectors

import (
	"bufio"
	"os"

	cclog "github.com/ClusterCockpit/cc-metric-collector/internal/ccLogger"
	lp "github.com/ClusterCockpit/cc-metric-collector/internal/ccMetric"

	//	"log"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"
)

const IOSTATFILE = `/proc/diskstats`
const IOSTAT_SYSFSPATH = `/sys/block`

type IOstatCollectorConfig struct {
	ExcludeMetrics []string `json:"exclude_metrics,omitempty"`
}

type IOstatCollectorEntry struct {
	lastValues map[string]int64
	tags       map[string]string
}

type IOstatCollector struct {
	metricCollector
	matches map[string]int
	config  IOstatCollectorConfig
	devices map[string]IOstatCollectorEntry
}

func (m *IOstatCollector) Init(config json.RawMessage) error {
	var err error
	m.name = "IOstatCollector"
	m.meta = map[string]string{"source": m.name, "group": "Disk"}
	m.setup()
	if len(config) > 0 {
		err = json.Unmarshal(config, &m.config)
		if err != nil {
			return err
		}
	}
	// https://www.kernel.org/doc/html/latest/admin-guide/iostats.html
	matches := map[string]int{
		"io_reads":             3,
		"io_reads_merged":      4,
		"io_read_sectors":      5,
		"io_read_ms":           6,
		"io_writes":            7,
		"io_writes_merged":     8,
		"io_writes_sectors":    9,
		"io_writes_ms":         10,
		"io_ioops":             11,
		"io_ioops_ms":          12,
		"io_ioops_weighted_ms": 13,
		"io_discards":          14,
		"io_discards_merged":   15,
		"io_discards_sectors":  16,
		"io_discards_ms":       17,
		"io_flushes":           18,
		"io_flushes_ms":        19,
	}
	m.devices = make(map[string]IOstatCollectorEntry)
	m.matches = make(map[string]int)
	for k, v := range matches {
		if _, skip := stringArrayContains(m.config.ExcludeMetrics, k); !skip {
			m.matches[k] = v
		}
	}
	if len(m.matches) == 0 {
		return errors.New("no metrics to collect")
	}
	file, err := os.Open(string(IOSTATFILE))
	if err != nil {
		cclog.ComponentError(m.name, err.Error())
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		linefields := strings.Fields(line)
		device := linefields[2]
		if strings.Contains(device, "loop") {
			continue
		}
		values := make(map[string]int64)
		for m := range m.matches {
			values[m] = 0
		}
		m.devices[device] = IOstatCollectorEntry{
			tags: map[string]string{
				"device": linefields[2],
				"type":   "node",
			},
			lastValues: values,
		}
	}
	m.init = true
	return err
}

func (m *IOstatCollector) Read(interval time.Duration, output chan lp.CCMetric) {
	if !m.init {
		return
	}

	file, err := os.Open(string(IOSTATFILE))
	if err != nil {
		cclog.ComponentError(m.name, err.Error())
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) == 0 {
			continue
		}
		linefields := strings.Fields(line)
		device := linefields[2]
		if strings.Contains(device, "loop") {
			continue
		}
		if _, ok := m.devices[device]; !ok {
			continue
		}
		entry := m.devices[device]
		for name, idx := range m.matches {
			if idx < len(linefields) {
				x, err := strconv.ParseInt(linefields[idx], 0, 64)
				if err == nil {
					diff := x - entry.lastValues[name]
					y, err := lp.New(name, entry.tags, m.meta, map[string]interface{}{"value": int(diff)}, time.Now())
					if err == nil {
						output <- y
					}
				}
				entry.lastValues[name] = x
			}
		}
		m.devices[device] = entry
	}
}

func (m *IOstatCollector) Close() {
	m.init = false
}
