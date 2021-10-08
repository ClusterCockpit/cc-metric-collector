package collectors

import (
	//	"errors"
	//	"fmt"
	lp "github.com/influxdata/line-protocol"
	"io/ioutil"
	"log"
	"strconv"
	"strings"
	"time"
)

const DISKSTATFILE = `/proc/diskstats`

type DiskstatCollector struct {
	MetricCollector
	matches map[int]string
}

func (m *DiskstatCollector) Init() error {
	m.name = "DiskstatCollector"
	m.setup()
	// https://www.kernel.org/doc/html/latest/admin-guide/iostats.html
	m.matches = map[int]string{
		3:  "reads",
		4:  "reads_merged",
		5:  "read_sectors",
		6:  "read_ms",
		7:  "writes",
		8:  "writes_merged",
		9:  "writes_sectors",
		10: "writes_ms",
		11: "ioops",
		12: "ioops_ms",
		13: "ioops_weighted_ms",
		14: "discards",
		15: "discards_merged",
		16: "discards_sectors",
		17: "discards_ms",
		18: "flushes",
		19: "flushes_ms",
	}
	_, err := ioutil.ReadFile(string(DISKSTATFILE))
	if err == nil {
		m.init = true
	}
	return err
}

func (m *DiskstatCollector) Read(interval time.Duration, out *[]lp.MutableMetric) {

	buffer, err := ioutil.ReadFile(string(DISKSTATFILE))

	if err != nil {
		log.Print(err)
		return
	}

	ll := strings.Split(string(buffer), "\n")

	for _, line := range ll {
		if len(line) == 0 {
			continue
		}
		f := strings.Fields(line)
		if strings.Contains(f[2], "loop") {
			continue
		}
		tags := map[string]string{
			"device": f[2],
			"type":   "node",
		}
		for idx, name := range m.matches {
			x, err := strconv.ParseInt(f[idx], 0, 64)
			if err == nil {
				y, err := lp.New(name, tags, map[string]interface{}{"value": int(x)}, time.Now())
				if err == nil {
					*out = append(*out, y)
				}
			}
		}
	}
	return
}

func (m *DiskstatCollector) Close() {
	m.init = false
	return
}
