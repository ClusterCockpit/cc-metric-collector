package collectors

import (
	"fmt"
	"io/ioutil"
	"log"
	"os/exec"

	lp "github.com/ClusterCockpit/cc-metric-collector/internal/ccMetric"

	//	"os"
	"encoding/json"
	"errors"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const PERFQUERY = `/usr/sbin/perfquery`

type InfinibandPerfQueryCollector struct {
	metricCollector
	tags   map[string]string
	lids   map[string]map[string]string
	config struct {
		ExcludeDevices []string `json:"exclude_devices,omitempty"`
		PerfQueryPath  string   `json:"perfquery_path"`
	}
}

func (m *InfinibandPerfQueryCollector) Init(config json.RawMessage) error {
	var err error
	m.name = "InfinibandCollectorPerfQuery"
	m.setup()
	m.meta = map[string]string{"source": m.name, "group": "Network"}
	m.tags = map[string]string{"type": "node"}
	if len(config) > 0 {
		err = json.Unmarshal(config, &m.config)
		if err != nil {
			return err
		}
	}
	if len(m.config.PerfQueryPath) == 0 {
		path, err := exec.LookPath("perfquery")
		if err == nil {
			m.config.PerfQueryPath = path
		}
	}
	m.lids = make(map[string]map[string]string)
	p := fmt.Sprintf("%s/*/ports/*/lid", string(IB_BASEPATH))
	files, err := filepath.Glob(p)
	for _, f := range files {
		lid, err := ioutil.ReadFile(f)
		if err == nil {
			plist := strings.Split(strings.Replace(f, string(IB_BASEPATH), "", -1), "/")
			skip := false
			for _, d := range m.config.ExcludeDevices {
				if d == plist[0] {
					skip = true
				}
			}
			if !skip {
				m.lids[plist[0]] = make(map[string]string)
				m.lids[plist[0]][plist[2]] = string(lid)
			}
		}
	}

	for _, ports := range m.lids {
		for port, lid := range ports {
			args := fmt.Sprintf("-r %s %s 0xf000", lid, port)
			command := exec.Command(m.config.PerfQueryPath, args)
			command.Wait()
			_, err := command.Output()
			if err != nil {
				return fmt.Errorf("Failed to execute %s: %v", m.config.PerfQueryPath, err)
			}
		}
	}

	if len(m.lids) == 0 {
		return errors.New("No usable IB devices")
	}

	m.init = true
	return nil
}

func (m *InfinibandPerfQueryCollector) doPerfQuery(cmd string, dev string, lid string, port string, tags map[string]string, output chan lp.CCMetric) error {

	args := fmt.Sprintf("-r %s %s 0xf000", lid, port)
	command := exec.Command(cmd, args)
	command.Wait()
	stdout, err := command.Output()
	if err != nil {
		log.Print(err)
		return err
	}
	ll := strings.Split(string(stdout), "\n")

	for _, line := range ll {
		if strings.HasPrefix(line, "PortRcvData") || strings.HasPrefix(line, "RcvData") {
			lv := strings.Fields(line)
			v, err := strconv.ParseFloat(lv[1], 64)
			if err == nil {
				y, err := lp.New("ib_recv", tags, m.meta, map[string]interface{}{"value": float64(v)}, time.Now())
				if err == nil {
					output <- y
				}
			}
		}
		if strings.HasPrefix(line, "PortXmitData") || strings.HasPrefix(line, "XmtData") {
			lv := strings.Fields(line)
			v, err := strconv.ParseFloat(lv[1], 64)
			if err == nil {
				y, err := lp.New("ib_xmit", tags, m.meta, map[string]interface{}{"value": float64(v)}, time.Now())
				if err == nil {
					output <- y
				}
			}
		}
		if strings.HasPrefix(line, "PortRcvPkts") || strings.HasPrefix(line, "RcvPkts") {
			lv := strings.Fields(line)
			v, err := strconv.ParseFloat(lv[1], 64)
			if err == nil {
				y, err := lp.New("ib_recv_pkts", tags, m.meta, map[string]interface{}{"value": float64(v)}, time.Now())
				if err == nil {
					output <- y
				}
			}
		}
		if strings.HasPrefix(line, "PortXmitPkts") || strings.HasPrefix(line, "XmtPkts") {
			lv := strings.Fields(line)
			v, err := strconv.ParseFloat(lv[1], 64)
			if err == nil {
				y, err := lp.New("ib_xmit_pkts", tags, m.meta, map[string]interface{}{"value": float64(v)}, time.Now())
				if err == nil {
					output <- y
				}
			}
		}
		if strings.HasPrefix(line, "PortRcvPkts") || strings.HasPrefix(line, "RcvPkts") {
			lv := strings.Fields(line)
			v, err := strconv.ParseFloat(lv[1], 64)
			if err == nil {
				y, err := lp.New("ib_recv_pkts", tags, m.meta, map[string]interface{}{"value": float64(v)}, time.Now())
				if err == nil {
					output <- y
				}
			}
		}
		if strings.HasPrefix(line, "PortXmitPkts") || strings.HasPrefix(line, "XmtPkts") {
			lv := strings.Fields(line)
			v, err := strconv.ParseFloat(lv[1], 64)
			if err == nil {
				y, err := lp.New("ib_xmit_pkts", tags, m.meta, map[string]interface{}{"value": float64(v)}, time.Now())
				if err == nil {
					output <- y
				}
			}
		}
	}
	return nil
}

func (m *InfinibandPerfQueryCollector) Read(interval time.Duration, output chan lp.CCMetric) {

	if m.init {
		for dev, ports := range m.lids {
			for port, lid := range ports {
				tags := map[string]string{
					"type":   "node",
					"device": dev,
					"port":   port,
					"lid":    lid}
				path := fmt.Sprintf("%s/%s/ports/%s/counters/", string(IB_BASEPATH), dev, port)
				buffer, err := ioutil.ReadFile(fmt.Sprintf("%s/port_rcv_data", path))
				if err == nil {
					data := strings.Replace(string(buffer), "\n", "", -1)
					v, err := strconv.ParseFloat(data, 64)
					if err == nil {
						y, err := lp.New("ib_recv", tags, m.meta, map[string]interface{}{"value": float64(v)}, time.Now())
						if err == nil {
							output <- y
						}
					}
				}
				buffer, err = ioutil.ReadFile(fmt.Sprintf("%s/port_xmit_data", path))
				if err == nil {
					data := strings.Replace(string(buffer), "\n", "", -1)
					v, err := strconv.ParseFloat(data, 64)
					if err == nil {
						y, err := lp.New("ib_xmit", tags, m.meta, map[string]interface{}{"value": float64(v)}, time.Now())
						if err == nil {
							output <- y
						}
					}
				}
				buffer, err = ioutil.ReadFile(fmt.Sprintf("%s/port_rcv_packets", path))
				if err == nil {
					data := strings.Replace(string(buffer), "\n", "", -1)
					v, err := strconv.ParseFloat(data, 64)
					if err == nil {
						y, err := lp.New("ib_recv_pkts", tags, m.meta, map[string]interface{}{"value": float64(v)}, time.Now())
						if err == nil {
							output <- y
						}
					}
				}
				buffer, err = ioutil.ReadFile(fmt.Sprintf("%s/port_xmit_packets", path))
				if err == nil {
					data := strings.Replace(string(buffer), "\n", "", -1)
					v, err := strconv.ParseFloat(data, 64)
					if err == nil {
						y, err := lp.New("ib_xmit_pkts", tags, m.meta, map[string]interface{}{"value": float64(v)}, time.Now())
						if err == nil {
							output <- y
						}
					}
				}
			}
		}
	}
}

func (m *InfinibandPerfQueryCollector) Close() {
	m.init = false
}
