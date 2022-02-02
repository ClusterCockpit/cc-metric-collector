package sinks

import (
	"fmt"
	"log"
	"strings"

	//	"time"
	"os/exec"

	lp "github.com/ClusterCockpit/cc-metric-collector/internal/ccMetric"
)

const GMETRIC_EXEC = `gmetric`

type GangliaSink struct {
	Sink
	gmetric_path string
}

func (s *GangliaSink) Init(config sinkConfig) error {
	p, err := exec.LookPath(string(GMETRIC_EXEC))
	if err == nil {
		s.gmetric_path = p
	}
	return err
}

func (s *GangliaSink) Write(point *lp.CCMetric) error {
	var err error = nil
	var tagsstr []string
	var argstr []string
  for key, value := range (*point).Tags() {
		switch key {
		case "cluster":
			argstr = append(argstr, fmt.Sprintf("--cluster=%s", value))
		case "unit":
			argstr = append(argstr, fmt.Sprintf("--units=%s", value))
		case "group":
			argstr = append(argstr, fmt.Sprintf("--group=%s", value))
		default:
			tagsstr = append(tagsstr, fmt.Sprintf("%s=%s", key, value))
		}
	}
	if len(tagsstr) > 0 {
		argstr = append(argstr, fmt.Sprintf("--desc=%q", strings.Join(tagsstr, ",")))
	}
	argstr = append(argstr, fmt.Sprintf("--name=%s", (*point).Name()))
	for _, f := range (*point).FieldList() {
		if f.Key == "value" {
			switch f.Value.(type) {
			case float64:
				argstr = append(argstr, fmt.Sprintf("--value=%v", f.Value.(float64)))
				argstr = append(argstr, "--type=double")
			case float32:
				argstr = append(argstr, fmt.Sprintf("--value=%v", f.Value.(float32)))
				argstr = append(argstr, "--type=float")
			case int:
				argstr = append(argstr, fmt.Sprintf("--value=%d", f.Value.(int)))
				argstr = append(argstr, "--type=int32")
			case int64:
				argstr = append(argstr, fmt.Sprintf("--value=%d", f.Value.(int64)))
				argstr = append(argstr, "--type=int32")
			case string:
				argstr = append(argstr, fmt.Sprintf("--value=%q", f.Value.(string)))
				argstr = append(argstr, "--type=string")
			}
		}
	}
	log.Print(s.gmetric_path, " ", strings.Join(argstr, " "))
	//	command := exec.Command(string(GMETRIC_EXEC), strings.Join(argstr, " "))
	//	command.Wait()
	//	_, err := command.Output()
	return err
}

func (s *GangliaSink) Flush() error {
	return nil
}

func (s *GangliaSink) Close() {
}
