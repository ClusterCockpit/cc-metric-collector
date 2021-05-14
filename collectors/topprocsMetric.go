package collectors

import (
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"
)

const NUM_PROCS = 5

type TopProcsCollector struct {
	MetricCollector
}

func (m *TopProcsCollector) Init() error {
	m.name = "TopProcsCollector"
	m.setup()
	return nil
}

func (m *TopProcsCollector) Read(interval time.Duration) {
	command := exec.Command("/usr/bin/ps", "-Ao", "comm", "--sort=-pcpu")
	command.Wait()
	stdout, err := command.Output()
	if err != nil {
		log.Print(m.name, err)
		return
	}

	lines := strings.Split(string(stdout), "\n")
	for i := 1; i < NUM_PROCS+1; i++ {
		m.node[fmt.Sprintf("topproc%d", i)] = lines[i]
	}
}

func (m *TopProcsCollector) Close() {
	return
}
