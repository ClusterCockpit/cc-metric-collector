package sinks

import (
	"fmt"
	"math"
	"strings"
	"time"
)

type StdoutSink struct {
	Sink
}

func (s *StdoutSink) Init(config SinkConfig) error {
	return nil
}

func (s *StdoutSink) Write(measurement string, tags map[string]string, fields map[string]interface{}, t time.Time) error {
	var tagsstr []string
	var fieldstr []string
	for k, v := range tags {
		tagsstr = append(tagsstr, fmt.Sprintf("%s=%s", k, v))
	}
	for k, v := range fields {
		switch v.(type) {
		case float64:
			if !math.IsNaN(v.(float64)) {
				fieldstr = append(fieldstr, fmt.Sprintf("%s=%v", k, v.(float64)))
			}
		case string:
			fieldstr = append(fieldstr, fmt.Sprintf("%s=%q", k, v.(string)))
		}
	}
	if len(tagsstr) > 0 {
		fmt.Printf("%s,%s %s %d\n", measurement, strings.Join(tagsstr, ","), strings.Join(fieldstr, ","), t.Unix())
	} else {
		fmt.Printf("%s %s %d\n", measurement, strings.Join(fieldstr, ","), t.Unix())
	}
	return nil
}

func (s *StdoutSink) Close() {
	return
}
