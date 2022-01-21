package sinks

import (
	"fmt"
	"math"
	"strings"

	//	"time"
	lp "github.com/influxdata/line-protocol"
)

type StdoutSink struct {
	Sink
}

func (s *StdoutSink) Init(config SinkConfig) error {
	return nil
}

func (s *StdoutSink) Write(point lp.MutableMetric) error {
	var tagsstr []string
	var fieldstr []string
	for _, t := range point.TagList() {
		tagsstr = append(tagsstr, fmt.Sprintf("%s=%s", t.Key, t.Value))
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
	return nil
}

func (s *StdoutSink) Close() {}
