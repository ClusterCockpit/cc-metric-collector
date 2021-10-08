package collectors

import (
	"fmt"
	lp "github.com/influxdata/line-protocol"
	"io/ioutil"
	"log"
	"os/exec"
	"time"
)

const CUSTOMCMDPATH = `/home/unrz139/Work/cc-metric-collector/collectors/custom`

type CustomCmdCollector struct {
	MetricCollector
	handler *lp.MetricHandler
	parser  *lp.Parser
}

func (m *CustomCmdCollector) Init() error {
	m.name = "CustomCmdCollector"
	m.setup()
	m.handler = lp.NewMetricHandler()
	m.parser = lp.NewParser(m.handler)
	m.parser.SetTimeFunc(DefaultTime)
	m.init = true
	return nil
}

var DefaultTime = func() time.Time {
	return time.Unix(42, 0)
}

func (m *CustomCmdCollector) Read(interval time.Duration, out *[]lp.MutableMetric) {
	files, err := ioutil.ReadDir(string(CUSTOMCMDPATH))
	if err != nil {
		log.Print(err)
		return
	}
	for _, file := range files {
		//		stat, err := os.Stat(file)
		//		if err != nil {
		//			log.Print(err)
		//			continue
		//		}
		//		mode := stat.Mode()
		//		if mode & 0o555 {
		path := fmt.Sprintf("%s/%s", string(CUSTOMCMDPATH), file.Name())
		command := exec.Command(path, "")
		command.Wait()
		stdout, err := command.Output()
		if err != nil {
			log.Print(err)
			continue
		}
		metrics, err := m.parser.Parse(stdout)
		if err != nil {
			log.Print(err)
			continue
		}
		for _, m := range metrics {
			y, err := lp.New(m.Name(), Tags2Map(m), Fields2Map(m), m.Time())
			if err == nil {
				*out = append(*out, y)
			}
			//				switch m.Name() {
			//				case "node":
			//					for k, v := range m.FieldList() {
			//						m.node[k] = float64(v)
			//					}
			//				case "socket":
			//					tlist := m.TagList()
			//					if id, found := tlist["socket"]; found {
			//						for k, v := range m.FieldList() {
			//							m.socket[id][k] = float64(v)
			//						}
			//					}
			//				case "cpu":
			//					tlist := m.TagList()
			//					if id, found := tlist["cpu"]; found {
			//						for k, v := range m.FieldList() {
			//							m.cpu[id][k] = float64(v)
			//						}
			//					}
			//				case "network":
			//					tlist := m.TagList()
			//					if id, found := tlist["device"]; found {
			//						for k, v := range m.FieldList() {
			//							m.network[id][k] = float64(v)
			//						}
			//					}
			//				case "accelerator":
			//					tlist := m.TagList()
			//					if id, found := tlist["device"]; found {
			//						for k, v := range m.FieldList() {
			//							m.accelerator[id][k] = float64(v)
			//						}
			//					}
			//				}
		}
		//		} if file is executable check
	}
}

func (m *CustomCmdCollector) Close() {
	m.init = false
	return
}
