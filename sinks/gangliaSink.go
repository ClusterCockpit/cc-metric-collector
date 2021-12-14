package sinks

import (
	"fmt"
	"strings"
    "log"
	//	"time"
	lp "github.com/influxdata/line-protocol"
	"os/exec"
)

const GMETRIC_EXEC = `gmetric`

type GangliaSink struct {
	Sink
	gmetric_path string
}

func (s *GangliaSink) Init(config SinkConfig) error {
    p, err := exec.LookPath(string(GMETRIC_EXEC))
    if err == nil {
        s.gmetric_path = p
    }
	return err
}

func (s *GangliaSink) Write(point lp.MutableMetric) error {
    var err error = nil
    var tagsstr []string
    var argstr []string
    for _, t := range point.TagList() {
        switch t.Key {
            case "cluster":
                argstr = append(argstr, fmt.Sprintf("--cluster=%s", t.Value))
            case "unit":
                argstr = append(argstr, fmt.Sprintf("--units=%s", t.Value))
            case "group":
                argstr = append(argstr, fmt.Sprintf("--group=%s", t.Value))
            default:
                tagsstr = append(tagsstr, fmt.Sprintf("%s=%s", t.Key, t.Value))
        }
	}
	if len(tagsstr) > 0 {
    	argstr = append(argstr, fmt.Sprintf("--desc=%q", strings.Join(tagsstr, ",")))
    }
	argstr = append(argstr, fmt.Sprintf("--name=%s", point.Name()))
	for _, f := range point.FieldList() {
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
	return
}
