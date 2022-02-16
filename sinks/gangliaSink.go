//go:build ganglia
// +build ganglia

package sinks

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	//	"time"
	"os/exec"

	cclog "github.com/ClusterCockpit/cc-metric-collector/internal/ccLogger"
	lp "github.com/ClusterCockpit/cc-metric-collector/internal/ccMetric"
)

const GMETRIC_EXEC = `gmetric`
const GMETRIC_CONFIG = `/etc/ganglia/gmond.conf`

type GangliaSinkConfig struct {
	defaultSinkConfig
	GmetricPath     string `json:"gmetric_path,omitempty"`
	GmetricConfig   string `json:"gmetric_config,omitempty"`
	AddGangliaGroup bool   `json:"add_ganglia_group,omitempty"`
	AddTagsAsDesc   bool   `json:"add_tags_as_desc,omitempty"`
}

type GangliaSink struct {
	sink
	gmetric_path   string
	gmetric_config string
	config         GangliaSinkConfig
}

func (s *GangliaSink) Init(config json.RawMessage) error {
	var err error = nil
	s.name = "GangliaSink"
	s.config.AddTagsAsDesc = false
	s.config.AddGangliaGroup = false
	if len(config) > 0 {
		err := json.Unmarshal(config, &s.config)
		if err != nil {
			cclog.ComponentError(s.name, "Error reading config for", s.name, ":", err.Error())
			return err
		}
	}
	s.gmetric_path = ""
	s.gmetric_config = ""
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
	if len(s.config.GmetricConfig) > 0 {
		s.gmetric_config = s.config.GmetricConfig
	}
	return err
}

func (s *GangliaSink) Write(point lp.CCMetric) error {
	var err error = nil
	var tagsstr []string
	var argstr []string
	if s.config.AddGangliaGroup {
		if point.HasTag("group") {
			g, _ := point.GetTag("group")
			argstr = append(argstr, fmt.Sprintf("--group=%s", g))
		} else if point.HasMeta("group") {
			g, _ := point.GetMeta("group")
			argstr = append(argstr, fmt.Sprintf("--group=%s", g))
		}
	}

	for key, value := range point.Tags() {
		switch key {
		case "cluster":
			argstr = append(argstr, fmt.Sprintf("--cluster=%s", value))
		case "unit":
			argstr = append(argstr, fmt.Sprintf("--units=%s", value))
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
			default:
				tagsstr = append(tagsstr, fmt.Sprintf("%s=%s", key, value))
			}
		}
	}
	if s.config.AddTagsAsDesc && len(tagsstr) > 0 {
		argstr = append(argstr, fmt.Sprintf("--desc=%q", strings.Join(tagsstr, ",")))
	}
	if len(s.gmetric_config) > 0 {
		argstr = append(argstr, fmt.Sprintf("--conf=%s", s.gmetric_config))
	}
	argstr = append(argstr, fmt.Sprintf("--name=%s", point.Name()))
	for k, v := range point.Fields() {
		if k == "value" {
			switch value := v.(type) {
			case float64:
				argstr = append(argstr,
					fmt.Sprintf("--value=%v", value), "--type=double")
			case float32:
				argstr = append(argstr,
					fmt.Sprintf("--value=%v", value), "--type=float")
			case int:
				argstr = append(argstr,
					fmt.Sprintf("--value=%d", value), "--type=int32")
			case int64:
				argstr = append(argstr,
					fmt.Sprintf("--value=%d", value), "--type=int32")
			case string:
				argstr = append(argstr,
					fmt.Sprintf("--value=%q", value), "--type=string")
			}
		}
	}
	command := exec.Command(s.gmetric_path, argstr...)
	command.Wait()
	_, err = command.Output()
	return err
}

func (s *GangliaSink) Flush() error {
	return nil
}

func (s *GangliaSink) Close() {
}
