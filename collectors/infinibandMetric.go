package collectors

import (
	"fmt"
	"io/ioutil"
	"log"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const LIDFILE = `/sys/class/infiniband/mlx4_0/ports/1/lid`
const PERFQUERY = `/usr/sbin/perfquery`

type InfinibandCollector struct {
	MetricCollector
}

func (m *InfinibandCollector) Init() error {
	m.name = "InfinibandCollector"
	m.setup()
	_, err := ioutil.ReadFile(string(LIDFILE))
	if err != nil {
		return err
	}
	_, err = ioutil.ReadFile(string(PERFQUERY))
	if err != nil {
		return err
	}
	return err
}

func (m *InfinibandCollector) Read(interval time.Duration) {
	buffer, err := ioutil.ReadFile(string(LIDFILE))

	if err != nil {
		log.Print(err)
		return
	}

	args := fmt.Sprintf("-r %s 1 0xf000", string(buffer))

	command := exec.Command(PERFQUERY, args)
	command.Wait()
	stdout, err := command.Output()
	if err != nil {
		log.Print(err)
		return
	}

	ll := strings.Split(string(stdout), "\n")

	for _, line := range ll {
		if strings.HasPrefix(line, "PortRcvData") || strings.HasPrefix(line, "RcvData") {
			lv := strings.Fields(line)
			v, err := strconv.ParseFloat(lv[1], 64)
			if err == nil {
				m.node["ib_recv"] = float64(v)
			}
		}
		if strings.HasPrefix(line, "PortXmitData") || strings.HasPrefix(line, "XmtData") {
			lv := strings.Fields(line)
			v, err := strconv.ParseFloat(lv[1], 64)
			if err == nil {
				m.node["ib_xmit"] = float64(v)
			}
		}
	}
}

func (m *InfinibandCollector) Close() {
	return
}
