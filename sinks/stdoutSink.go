package sinks

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	//	"time"
	cclog "github.com/ClusterCockpit/cc-metric-collector/pkg/ccLogger"
	lp "github.com/ClusterCockpit/cc-energy-manager/pkg/cc-message"
)

type StdoutSink struct {
	sink   // meta_as_tags, name
	output *os.File
	config struct {
		defaultSinkConfig
		Output string `json:"output_file,omitempty"`
	}
}

func (s *StdoutSink) Write(m lp.CCMessage) error {
	fmt.Fprint(
		s.output,
		m.ToLineProtocol(s.meta_as_tags),
	)
	return nil
}

func (s *StdoutSink) Flush() error {
	s.output.Sync()
	return nil
}

func (s *StdoutSink) Close() {
	if s.output != os.Stdout && s.output != os.Stderr {
		s.output.Close()
	}
}

func NewStdoutSink(name string, config json.RawMessage) (Sink, error) {
	s := new(StdoutSink)
	s.name = fmt.Sprintf("StdoutSink(%s)", name)
	if len(config) > 0 {
		d := json.NewDecoder(bytes.NewReader(config))
		d.DisallowUnknownFields()
		if err := d.Decode(&s.config); err != nil {
			cclog.ComponentError(s.name, "Error reading config:", err.Error())
			return nil, err
		}
	}

	s.output = os.Stdout
	if len(s.config.Output) > 0 {
		switch strings.ToLower(s.config.Output) {
		case "stdout":
			s.output = os.Stdout
		case "stderr":
			s.output = os.Stderr
		default:
			f, err := os.OpenFile(s.config.Output, os.O_CREATE|os.O_WRONLY, os.FileMode(0600))
			if err != nil {
				return nil, err
			}
			s.output = f
		}
	}
	// Create lookup map to use meta infos as tags in the output metric
	s.meta_as_tags = make(map[string]bool)
	for _, k := range s.config.MetaAsTags {
		s.meta_as_tags[k] = true
	}

	return s, nil
}
