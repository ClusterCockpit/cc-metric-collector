package collectors

import (
	"encoding/json"
	lp "github.com/influxdata/line-protocol"
	"io/ioutil"
	"log"
	"strconv"
	"strings"
	"time"
)

const NETSTATFILE = `/proc/net/dev`

type NetstatCollectorConfig struct {
	ExcludeDevices []string `json:"exclude_devices"`
}

type NetstatCollector struct {
	MetricCollector
	config  NetstatCollectorConfig
	matches map[int]string
}

func (m *NetstatCollector) Init(config []byte) error {
	m.name = "NetstatCollector"
	m.setup()
	m.matches = map[int]string{
		1:  "bytes_in",
		9:  "bytes_out",
		2:  "pkts_in",
		10: "pkts_out",
	}
	if len(config) > 0 {
	    err := json.Unmarshal(config, &m.config)
	    if err != nil {
		    log.Print(err.Error())
		    return err
	    }
	}
	_, err := ioutil.ReadFile(string(NETSTATFILE))
	if err == nil {
		m.init = true
	}
	return nil
}

func (m *NetstatCollector) Read(interval time.Duration, out *[]lp.MutableMetric) {
	data, err := ioutil.ReadFile(string(NETSTATFILE))
	if err != nil {
		log.Print(err.Error())
		return
	}

	lines := strings.Split(string(data), "\n")
	for _, l := range lines {
		if !strings.Contains(l, ":") {
			continue
		}
		f := strings.Fields(l)
		dev := f[0][0 : len(f[0])-1]
		cont := false
		for _, d := range m.config.ExcludeDevices {
			if d == dev {
				cont = true
			}
		}
		if cont {
			continue
		}
		tags := map[string]string{"device": dev, "type": "node"}
		for i, name := range m.matches {
			v, err := strconv.ParseInt(f[i], 10, 0)
			if err == nil {
				y, err := lp.New(name, tags, map[string]interface{}{"value": int(float64(v) * 1.0e-3)}, time.Now())
				if err == nil {
					*out = append(*out, y)
				}
			}
		}
	}

}

func (m *NetstatCollector) Close() {
	m.init = false
	return
}
