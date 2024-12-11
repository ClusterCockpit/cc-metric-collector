package sinks

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	//	"time"
	"os/exec"

	lp "github.com/ClusterCockpit/cc-energy-manager/pkg/cc-message"
	cclog "github.com/ClusterCockpit/cc-metric-collector/pkg/ccLogger"
	mp "github.com/ClusterCockpit/cc-metric-collector/pkg/messageProcessor"
)

const GMETRIC_EXEC = `gmetric`
const GMETRIC_CONFIG = `/etc/ganglia/gmond.conf`

type GangliaSinkConfig struct {
	defaultSinkConfig
	GmetricPath     string `json:"gmetric_path,omitempty"`
	GmetricConfig   string `json:"gmetric_config,omitempty"`
	AddGangliaGroup bool   `json:"add_ganglia_group,omitempty"`
	AddTagsAsDesc   bool   `json:"add_tags_as_desc,omitempty"`
	ClusterName     string `json:"cluster_name,omitempty"`
	AddTypeToName   bool   `json:"add_type_to_name,omitempty"`
	AddUnits        bool   `json:"add_units,omitempty"`
}

type GangliaSink struct {
	sink
	gmetric_path   string
	gmetric_config string
	config         GangliaSinkConfig
}

func (s *GangliaSink) Write(msg lp.CCMessage) error {
	var err error = nil
	//var tagsstr []string
	var argstr []string

	point, err := s.mp.ProcessMessage(msg)
	if err == nil && point != nil {
		// Get metric config (type, value, ... in suitable format)
		conf := GetCommonGangliaConfig(point)
		if len(conf.Type) == 0 {
			conf = GetGangliaConfig(point)
		}
		if len(conf.Type) == 0 {
			return fmt.Errorf("metric %q (Ganglia name %q) has no 'value' field", point.Name(), conf.Name)
		}

		if s.config.AddGangliaGroup {
			argstr = append(argstr, fmt.Sprintf("--group=%s", conf.Group))
		}
		if s.config.AddUnits && len(conf.Unit) > 0 {
			argstr = append(argstr, fmt.Sprintf("--units=%s", conf.Unit))
		}

		if len(s.config.ClusterName) > 0 {
			argstr = append(argstr, fmt.Sprintf("--cluster=%s", s.config.ClusterName))
		}
		// if s.config.AddTagsAsDesc && len(tagsstr) > 0 {
		// 	argstr = append(argstr, fmt.Sprintf("--desc=%q", strings.Join(tagsstr, ",")))
		// }
		if len(s.gmetric_config) > 0 {
			argstr = append(argstr, fmt.Sprintf("--conf=%s", s.gmetric_config))
		}
		if s.config.AddTypeToName {
			argstr = append(argstr, fmt.Sprintf("--name=%s", GangliaMetricName(point)))
		} else {
			argstr = append(argstr, fmt.Sprintf("--name=%s", conf.Name))
		}
		argstr = append(argstr, fmt.Sprintf("--slope=%s", conf.Slope))
		argstr = append(argstr, fmt.Sprintf("--value=%s", conf.Value))
		argstr = append(argstr, fmt.Sprintf("--type=%s", conf.Type))
		argstr = append(argstr, fmt.Sprintf("--tmax=%d", conf.Tmax))

		cclog.ComponentDebug(s.name, s.gmetric_path, strings.Join(argstr, " "))
		command := exec.Command(s.gmetric_path, argstr...)
		command.Wait()
		_, err = command.Output()
	}
	return err
}

func (s *GangliaSink) Flush() error {
	return nil
}

func (s *GangliaSink) Close() {
}

func NewGangliaSink(name string, config json.RawMessage) (Sink, error) {
	s := new(GangliaSink)
	s.name = fmt.Sprintf("GangliaSink(%s)", name)
	s.config.AddTagsAsDesc = false
	s.config.AddGangliaGroup = false
	if len(config) > 0 {
		d := json.NewDecoder(bytes.NewReader(config))
		d.DisallowUnknownFields()
		if err := d.Decode(&s.config); err != nil {
			cclog.ComponentError(s.name, "Error reading config:", err.Error())
			return nil, err
		}
	}
	s.gmetric_path = ""
	s.gmetric_config = ""

	p, err := mp.NewMessageProcessor()
	if err != nil {
		return nil, fmt.Errorf("initialization of message processor failed: %v", err.Error())
	}
	s.mp = p

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
		return nil, errors.New("cannot find executable 'gmetric'")
	}
	if len(s.config.GmetricConfig) > 0 {
		s.gmetric_config = s.config.GmetricConfig
	}
	if len(s.config.MessageProcessor) > 0 {
		err = s.mp.FromConfigJSON(s.config.MessageProcessor)
		if err != nil {
			return nil, fmt.Errorf("failed parsing JSON for message processor: %v", err.Error())
		}
	}
	for _, k := range s.config.MetaAsTags {
		s.mp.AddMoveMetaToTags("true", k, k)
	}

	return s, nil
}
