package sinks

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"

	//	"time"
	"os/exec"

	cclog "github.com/ClusterCockpit/cc-metric-collector/internal/ccLogger"
	lp "github.com/ClusterCockpit/cc-metric-collector/internal/ccMetric"
)

const GMETRIC_EXEC = `gmetric`

type GangliaSinkConfig struct {
	defaultSinkConfig
	GmetricPath     string `json:"gmetric_path"`
	AddGangliaGroup bool   `json:"add_ganglia_group"`
}

type GangliaSink struct {
	sink
	gmetric_path string
	config       GangliaSinkConfig
}

func (s *GangliaSink) Init(config json.RawMessage) error {
	var err error = nil
	s.name = "GangliaSink"
	if len(config) > 0 {
		err := json.Unmarshal(config, &s.config)
		if err != nil {
			cclog.ComponentError(s.name, "Error reading config for", s.name, ":", err.Error())
			return err
		}
	}
	s.gmetric_path = ""
	if len(s.config.GmetricPath) > 0 {
		p, err := exec.LookPath(s.config.GmetricPath)
		if err == nil {
			s.gmetric_path = p
		}
	}
	if len(s.gmetric_path) == 0 {
		p, err := exec.LookPath(string(GMETRIC_EXEC))
		if err == nil {
			s.gmetric_path = p
		}
	}
	if len(s.gmetric_path) == 0 {
		err = errors.New("cannot find executable 'gmetric'")
	}
	return err
}

func (s *GangliaSink) Write(point lp.CCMetric) error {
	var err error = nil
	var tagsstr []string
	var argstr []string
	for key, value := range point.Tags() {
		switch key {
		case "cluster":
			argstr = append(argstr, fmt.Sprintf("--cluster=%s", value))
		case "unit":
			argstr = append(argstr, fmt.Sprintf("--units=%s", value))
		case "group":
			if s.config.AddGangliaGroup {
				argstr = append(argstr, fmt.Sprintf("--group=%s", value))
			}
		default:
			tagsstr = append(tagsstr, fmt.Sprintf("%s=%s", key, value))
		}
	}
	if s.config.MetaAsTags {
		for key, value := range point.Meta() {
			switch key {
			case "cluster":
				argstr = append(argstr, fmt.Sprintf("--cluster=%s", value))
			case "unit":
				argstr = append(argstr, fmt.Sprintf("--units=%s", value))
			case "group":
				if s.config.AddGangliaGroup {
					argstr = append(argstr, fmt.Sprintf("--group=%s", value))
				}
			default:
				tagsstr = append(tagsstr, fmt.Sprintf("%s=%s", key, value))
			}
		}
	}
	if len(tagsstr) > 0 {
		argstr = append(argstr, fmt.Sprintf("--desc=%q", strings.Join(tagsstr, ",")))
	}
	argstr = append(argstr, fmt.Sprintf("--name=%s", point.Name()))
	for k, v := range point.Fields() {
		if k == "value" {
			switch value := v.(type) {
			case float64:
				argstr = append(argstr, fmt.Sprintf("--value=%v", value))
				argstr = append(argstr, "--type=double")
			case float32:
				argstr = append(argstr, fmt.Sprintf("--value=%v", value))
				argstr = append(argstr, "--type=float")
			case int:
				argstr = append(argstr, fmt.Sprintf("--value=%d", value))
				argstr = append(argstr, "--type=int32")
			case int64:
				argstr = append(argstr, fmt.Sprintf("--value=%d", value))
				argstr = append(argstr, "--type=int32")
			case string:
				argstr = append(argstr, fmt.Sprintf("--value=%q", value))
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
