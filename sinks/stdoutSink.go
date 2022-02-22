package sinks

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	//	"time"
	lp "github.com/ClusterCockpit/cc-metric-collector/internal/ccMetric"
)

type StdoutSink struct {
	sink   // meta_as_tags, name
	output *os.File
	config struct {
		defaultSinkConfig
		Output string `json:"output_file,omitempty"`
	}
}

func (s *StdoutSink) Init(name string, config json.RawMessage) error {
	s.name = fmt.Sprintf("StdoutSink(%s)", name)
	if len(config) > 0 {
		err := json.Unmarshal(config, &s.config)
		if err != nil {
			return err
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
				return err
			}
			s.output = f
		}
	}
	s.meta_as_tags = s.config.MetaAsTags
	return nil
}

func (s *StdoutSink) Write(m lp.CCMetric) error {
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
	s.Init(name, config)
	return s, nil
}
