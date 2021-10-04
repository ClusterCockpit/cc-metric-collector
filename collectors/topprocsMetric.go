package collectors

import (
	"fmt"
	lp "github.com/influxdata/line-protocol"
	"log"
	"os/exec"
	"strings"
	"time"
)

const NUM_PROCS = 5

type TopProcsCollector struct {
	MetricCollector
	tags map[string]string
}

func (m *TopProcsCollector) Init() error {
	m.name = "TopProcsCollector"
	m.tags = map[string]string{"type": "node"}
	m.setup()
	command := exec.Command("ps", "-Ao", "comm", "--sort=-pcpu")
	command.Wait()
	_, err := command.Output()
	if err == nil {
	    m.init = true
	}
	return nil
}

func (m *TopProcsCollector) Read(interval time.Duration, out *[]lp.MutableMetric) {
	command := exec.Command("ps", "-Ao", "comm", "--sort=-pcpu")
	command.Wait()
	stdout, err := command.Output()
	if err != nil {
		log.Print(m.name, err)
		return
	}

	lines := strings.Split(string(stdout), "\n")
	for i := 1; i < NUM_PROCS+1; i++ {
		name := fmt.Sprintf("topproc%d", i)
		y, err := lp.New(name, m.tags, map[string]interface{}{"value": string(lines[i])}, time.Now())
		if err == nil {
			*out = append(*out, y)
		}
	}
}

func (m *TopProcsCollector) Close() {
    m.init = false
	return
}
