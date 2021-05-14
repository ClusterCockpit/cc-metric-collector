package collectors

import (
	"io/ioutil"
	"log"
	"strconv"
	"strings"
	"time"
)

const LUSTREFILE = `/proc/fs/lustre/llite/lnec-XXXXXX/stats`

type LustreCollector struct {
	MetricCollector
}

func (m *LustreCollector) Init() error {
	m.name = "LustreCollector"
	m.setup()
	_, err := ioutil.ReadFile(string(LUSTREFILE))
	return err
}

func (m *LustreCollector) Read(interval time.Duration) {
	buffer, err := ioutil.ReadFile(string(LUSTREFILE))

	if err != nil {
		log.Print(err)
		return
	}

	for _, line := range strings.Split(string(buffer), "\n") {
		lf := strings.Fields(line)
		if len(lf) > 1 {
			switch lf[0] {
			case "read_bytes":
				m.node["read_bytes"], err = strconv.ParseInt(lf[6], 0, 64)
				m.node["read_requests"], err = strconv.ParseInt(lf[1], 0, 64)
			case "write_bytes":
				m.node["write_bytes"], err = strconv.ParseInt(lf[6], 0, 64)
				m.node["write_requests"], err = strconv.ParseInt(lf[1], 0, 64)
			case "open":
				m.node["open"], err = strconv.ParseInt(lf[1], 0, 64)
			case "close":
				m.node["close"], err = strconv.ParseInt(lf[1], 0, 64)
			case "setattr":
				m.node["setattr"], err = strconv.ParseInt(lf[1], 0, 64)
			case "getattr":
				m.node["getattr"], err = strconv.ParseInt(lf[1], 0, 64)
			case "statfs":
				m.node["statfs"], err = strconv.ParseInt(lf[1], 0, 64)
			case "inode_permission":
				m.node["inode_permission"], err = strconv.ParseInt(lf[1], 0, 64)
			}

		}
	}
}

func (m *LustreCollector) Close() {
	return
}
