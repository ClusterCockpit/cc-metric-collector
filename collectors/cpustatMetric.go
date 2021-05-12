package collectors

import (
	"io/ioutil"
	"strconv"
	"strings"
	"time"
)

const CPUSTATFILE = `/proc/stat`

type CpustatCollector struct {
	MetricCollector
}

func (m *CpustatCollector) Init() {
	m.name = "CpustatCollector"
	m.setup()
}

func ParseStatLine(line string, out map[string]interface{}) {
	ls := strings.Fields(line)
	user, _ := strconv.ParseInt(ls[1], 0, 64)
	out["cpu_user"] = float64(user)
	nice, _ := strconv.ParseInt(ls[2], 0, 64)
	out["cpu_nice"] = float64(nice)
	system, _ := strconv.ParseInt(ls[3], 0, 64)
	out["cpu_system"] = float64(system)
	idle, _ := strconv.ParseInt(ls[4], 0, 64)
	out["cpu_idle"] = float64(idle)
	iowait, _ := strconv.ParseInt(ls[5], 0, 64)
	out["cpu_iowait"] = float64(iowait)
	irq, _ := strconv.ParseInt(ls[6], 0, 64)
	out["cpu_irq"] = float64(irq)
	softirq, _ := strconv.ParseInt(ls[7], 0, 64)
	out["cpu_softirq"] = float64(softirq)
	steal, _ := strconv.ParseInt(ls[8], 0, 64)
	out["cpu_steal"] = float64(steal)
	guest, _ := strconv.ParseInt(ls[9], 0, 64)
	out["cpu_guest"] = float64(guest)
	guest_nice, _ := strconv.ParseInt(ls[10], 0, 64)
	out["cpu_guest_nice"] = float64(guest_nice)
}

func (m *CpustatCollector) Read(interval time.Duration) {
	buffer, err := ioutil.ReadFile(string(CPUSTATFILE))

	if err != nil {
		return
	}

	ll := strings.Split(string(buffer), "\n")
	for _, line := range ll {
		if len(line) == 0 {
			continue
		}
		ls := strings.Fields(line)
		if strings.Compare(ls[0], "cpu") == 0 {
			ParseStatLine(line, m.node)
		} else if strings.HasPrefix(ls[0], "cpu") {
			cpustr := strings.TrimLeft(ls[0], "cpu")
			cpu, _ := strconv.Atoi(cpustr)
			ParseStatLine(line, m.cpus[cpu])
		}
	}
}

func (m *CpustatCollector) Close() {
	return
}
