package sinks

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"

	//	"time"
	lp "github.com/ClusterCockpit/cc-metric-collector/internal/ccMetric"
)

type StdoutSinkConfig struct {
	defaultSinkConfig
	Output string `json:"output_file,omitempty"`
}

type StdoutSink struct {
	sink
	output *os.File
	config StdoutSinkConfig
}

func (s *StdoutSink) Init(config json.RawMessage) error {
	s.name = "StdoutSink"
	if len(config) > 0 {
		err := json.Unmarshal(config, &s.config)
		if err != nil {
			return err
		}
	}
	s.output = os.Stdout
	if len(s.config.Output) > 0 {
		if strings.ToLower(s.config.Output) == "stdout" {
			s.output = os.Stdout
		} else if strings.ToLower(s.config.Output) == "stderr" {
			s.output = os.Stderr
		} else {
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

func (s *StdoutSink) Write(point lp.CCMetric) error {
	var tagsstr []string
	var fieldstr []string
	for _, t := range point.TagList() {
		tagsstr = append(tagsstr, fmt.Sprintf("%s=%s", t.Key, t.Value))
	}
	if s.meta_as_tags {
		for _, m := range point.MetaList() {
			tagsstr = append(tagsstr, fmt.Sprintf("%s=%s", m.Key, m.Value))
		}
	}
	for _, f := range point.FieldList() {
		switch f.Value.(type) {
		case float64:
			if !math.IsNaN(f.Value.(float64)) {
				fieldstr = append(fieldstr, fmt.Sprintf("%s=%v", f.Key, f.Value.(float64)))
			} else {
				fieldstr = append(fieldstr, fmt.Sprintf("%s=0.0", f.Key))
			}
		case float32:
			if !math.IsNaN(float64(f.Value.(float32))) {
				fieldstr = append(fieldstr, fmt.Sprintf("%s=%v", f.Key, f.Value.(float32)))
			} else {
				fieldstr = append(fieldstr, fmt.Sprintf("%s=0.0", f.Key))
			}
		case int:
			fieldstr = append(fieldstr, fmt.Sprintf("%s=%d", f.Key, f.Value.(int)))
		case int64:
			fieldstr = append(fieldstr, fmt.Sprintf("%s=%d", f.Key, f.Value.(int64)))
		case string:
			fieldstr = append(fieldstr, fmt.Sprintf("%s=%q", f.Key, f.Value.(string)))
		default:
			fieldstr = append(fieldstr, fmt.Sprintf("%s=%v", f.Key, f.Value))
		}
	}
	if len(tagsstr) > 0 {
		fmt.Printf("%s,%s %s %d\n", point.Name(), strings.Join(tagsstr, ","), strings.Join(fieldstr, ","), point.Time().Unix())
	} else {
		fmt.Printf("%s %s %d\n", point.Name(), strings.Join(fieldstr, ","), point.Time().Unix())
	}
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
