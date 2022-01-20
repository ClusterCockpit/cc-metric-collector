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

const IBBASEPATH = `/sys/class/infiniband/`
const LIDFILE = `/sys/class/infiniband/mlx4_0/ports/1/lid`
const PERFQUERY = `/usr/sbin/perfquery`

type InfinibandCollectorConfig struct {
	ExcludeDevices []string `json:"exclude_devices,omitempty"`
	PerfQueryPath  string   `json:"perfquery_path"`
}

type InfinibandCollector struct {
	metricCollector
	tags          map[string]string
	lids          map[string]map[string]string
	config        InfinibandCollectorConfig
	use_perfquery bool
}

func (m *InfinibandCollector) Help() {
	fmt.Println("This collector includes all devices that can be found below ", IBBASEPATH)
	fmt.Println("and where any of the ports provides a 'lid' file (glob ", IBBASEPATH, "/<dev>/ports/<port>/lid).")
	fmt.Println("The devices can be filtered with the 'exclude_devices' option in the configuration.")
	fmt.Println("For each found LIDs the collector calls the 'perfquery' command")
	fmt.Println("The path to the 'perfquery' command can be configured with the 'perfquery_path' option")
	fmt.Println("in the configuration\n")
	fmt.Println("Full configuration object:")
	fmt.Println("\"ibstat\" : {")
	fmt.Println("  \"perfquery_path\" : \"path/to/perfquery\"  # if omitted, it searches in $PATH")
	fmt.Println("  \"exclude_devices\" : [\"dev1\"]")
	fmt.Println("}\n")
	fmt.Println("Metrics:")
	fmt.Println("- ib_recv")
	fmt.Println("- ib_xmit")
	fmt.Println("- ib_recv_pkts")
	fmt.Println("- ib_xmit_pkts")
}

func (m *InfinibandCollector) Init(config json.RawMessage) error {
	var err error
	m.name = "InfinibandCollector"
	m.use_perfquery = false
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
	p := fmt.Sprintf("%s/*/ports/*/lid", string(IBBASEPATH))
	files, err := filepath.Glob(p)
	for _, f := range files {
		lid, err := ioutil.ReadFile(f)
		if err == nil {
			plist := strings.Split(strings.Replace(f, string(IBBASEPATH), "", -1), "/")
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

func (m *InfinibandCollector) doPerfQuery(cmd string, dev string, lid string, port string, tags map[string]string, output chan lp.CCMetric) error {

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
	}
	return nil
}

func (m *InfinibandCollector) doSysfsRead(dev string, lid string, port string, tags map[string]string, output chan lp.CCMetric) error {
	path := fmt.Sprintf("%s/%s/ports/%s/counters/", string(IBBASEPATH), dev, port)
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

	return nil
}

func (m *InfinibandCollector) Read(interval time.Duration, output chan lp.CCMetric) {

	if m.init {
		for dev, ports := range m.lids {
			for port, lid := range ports {
				tags := map[string]string{"type": "node", "device": dev, "port": port}
				if m.use_perfquery {
					m.doPerfQuery(m.config.PerfQueryPath, dev, lid, port, tags, output)
				} else {
					m.doSysfsRead(dev, lid, port, tags, output)
				}
			}
		}
	}
}

func (m *InfinibandCollector) Close() {
	m.init = false
	return
}
