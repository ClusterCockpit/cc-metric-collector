package collectors

import (
	lp "github.com/influxdata/line-protocol"
	"io/ioutil"
	"log"
	"strconv"
	"strings"
	"time"
)

const LUSTREFILE = `/proc/fs/lustre/llite/lnec-XXXXXX/stats`

type LustreCollector struct {
	MetricCollector
	tags    map[string]string
	matches map[string]map[string]int
}

func (m *LustreCollector) Init() error {
	m.name = "LustreCollector"
	m.setup()
	m.tags = map[string]string{"type": "node"}
	m.matches = map[string]map[string]int{"read_bytes": {"read_bytes": 6, "read_requests": 1},
		"write_bytes":      {"write_bytes": 6, "write_requests": 1},
		"open":             {"open": 1},
		"close":            {"close": 1},
		"setattr":          {"setattr": 1},
		"getattr":          {"getattr": 1},
		"statfs":           {"statfs": 1},
		"inode_permission": {"inode_permission": 1}}
	_, err := ioutil.ReadFile(string(LUSTREFILE))
	if err == nil {
	    m.init = true
	}
	return err
}

func (m *LustreCollector) Read(interval time.Duration, out *[]lp.MutableMetric) {
	buffer, err := ioutil.ReadFile(string(LUSTREFILE))

	if err != nil {
		log.Print(err)
		return
	}

	for _, line := range strings.Split(string(buffer), "\n") {
		lf := strings.Fields(line)
		if len(lf) > 1 {
			for match, fields := range m.matches {
				if lf[0] == match {
					for name, idx := range fields {
						x, err := strconv.ParseInt(lf[idx], 0, 64)
						if err == nil {
							y, err := lp.New(name, m.tags, map[string]interface{}{"value": x}, time.Now())
							if err == nil {
								*out = append(*out, y)
							}
						}
					}
				}
			}
		}
	}
}

func (m *LustreCollector) Close() {
    m.init = false
	return
}
