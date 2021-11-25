package collectors

import (
	"fmt"
	lp "github.com/influxdata/line-protocol"
	"io/ioutil"
	"log"
	"os/exec"
	//	"os"
	"encoding/json"
	"errors"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const BASEPATH = `/sys/class/infiniband/`
const LIDFILE = `/sys/class/infiniband/mlx4_0/ports/1/lid`
const PERFQUERY = `/usr/sbin/perfquery`

type InfinibandCollectorConfig struct {
	ExcludeDevices []string `json:"exclude_devices, omitempty"`
}

type InfinibandCollector struct {
	MetricCollector
	tags          map[string]string
	lids          map[string]map[string]string
	config        NetstatCollectorConfig
	use_perfquery bool
}

func (m *InfinibandCollector) Init(config []byte) error {
	var err error
	m.name = "InfinibandCollector"
	m.use_perfquery = false
	m.setup()
	m.tags = map[string]string{"type": "node"}
	if len(config) > 0 {
		err = json.Unmarshal(config, &m.config)
		if err != nil {
			return err
		}
	}
	m.lids = make(map[string]map[string]string)
	p := fmt.Sprintf("%s/*/ports/*/lid", string(BASEPATH))
	files, err := filepath.Glob(p)
	for _, f := range files {
		lid, err := ioutil.ReadFile(f)
		if err == nil {
			plist := strings.Split(strings.Replace(f, string(BASEPATH), "", -1), "/")
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
			command := exec.Command(PERFQUERY, args)
			command.Wait()
			_, err := command.Output()
			if err == nil {
				m.use_perfquery = true
			}
			break
		}
		break
	}

	if len(m.lids) > 0 {
		m.init = true
	} else {
		err = errors.New("No usable devices")
	}

	return err
}

func DoPerfQuery(dev string, lid string, port string, tags map[string]string, out *[]lp.MutableMetric) error {

	args := fmt.Sprintf("-r %s %s 0xf000", lid, port)
	command := exec.Command(PERFQUERY, args)
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
				y, err := lp.New("ib_recv", tags, map[string]interface{}{"value": float64(v)}, time.Now())
				if err == nil {
					*out = append(*out, y)
				}
			}
		}
		if strings.HasPrefix(line, "PortXmitData") || strings.HasPrefix(line, "XmtData") {
			lv := strings.Fields(line)
			v, err := strconv.ParseFloat(lv[1], 64)
			if err == nil {
				y, err := lp.New("ib_xmit", tags, map[string]interface{}{"value": float64(v)}, time.Now())
				if err == nil {
					*out = append(*out, y)
				}
			}
		}
	}
	return nil
}

func DoSysfsRead(dev string, lid string, port string, tags map[string]string, out *[]lp.MutableMetric) error {
	path := fmt.Sprintf("%s/%s/ports/%s/counters/", string(BASEPATH), dev, port)
	buffer, err := ioutil.ReadFile(fmt.Sprintf("%s/port_rcv_data", path))
	if err == nil {
		data := strings.Replace(string(buffer), "\n", "", -1)
		v, err := strconv.ParseFloat(data, 64)
		if err == nil {
			y, err := lp.New("ib_recv", tags, map[string]interface{}{"value": float64(v)}, time.Now())
			if err == nil {
				*out = append(*out, y)
			}
		}
	}
	buffer, err = ioutil.ReadFile(fmt.Sprintf("%s/port_xmit_data", path))
	if err == nil {
		data := strings.Replace(string(buffer), "\n", "", -1)
		v, err := strconv.ParseFloat(data, 64)
		if err == nil {
			y, err := lp.New("ib_xmit", tags, map[string]interface{}{"value": float64(v)}, time.Now())
			if err == nil {
				*out = append(*out, y)
			}
		}
	}
	return nil
}

func (m *InfinibandCollector) Read(interval time.Duration, out *[]lp.MutableMetric) {

	if m.init {
		for dev, ports := range m.lids {
			for port, lid := range ports {
				tags := map[string]string{"type": "node", "device": dev, "port": port}
				if m.use_perfquery {
					DoPerfQuery(dev, lid, port, tags, out)
				} else {
					DoSysfsRead(dev, lid, port, tags, out)
				}
			}
		}
	}

	//	buffer, err := ioutil.ReadFile(string(LIDFILE))

	//	if err != nil {
	//		log.Print(err)
	//		return
	//	}

	//	args := fmt.Sprintf("-r %s 1 0xf000", string(buffer))

	//	command := exec.Command(PERFQUERY, args)
	//	command.Wait()
	//	stdout, err := command.Output()
	//	if err != nil {
	//		log.Print(err)
	//		return
	//	}

	//	ll := strings.Split(string(stdout), "\n")

	//	for _, line := range ll {
	//		if strings.HasPrefix(line, "PortRcvData") || strings.HasPrefix(line, "RcvData") {
	//			lv := strings.Fields(line)
	//			v, err := strconv.ParseFloat(lv[1], 64)
	//			if err == nil {
	//				y, err := lp.New("ib_recv", m.tags, map[string]interface{}{"value": float64(v)}, time.Now())
	//				if err == nil {
	//					*out = append(*out, y)
	//				}
	//			}
	//		}
	//		if strings.HasPrefix(line, "PortXmitData") || strings.HasPrefix(line, "XmtData") {
	//			lv := strings.Fields(line)
	//			v, err := strconv.ParseFloat(lv[1], 64)
	//			if err == nil {
	//				y, err := lp.New("ib_xmit", m.tags, map[string]interface{}{"value": float64(v)}, time.Now())
	//				if err == nil {
	//					*out = append(*out, y)
	//				}
	//			}
	//		}
	//	}
}

func (m *InfinibandCollector) Close() {
	m.init = false
	return
}
