package sinks

import (
    "fmt"
    "time"
    "strings"
    "math"
)

type StdoutSink struct {
	Sink
}

func (s *StdoutSink) Init(host string, port string, user string, password string, database string) error {
	s.host = host
	s.port = port
	s.user = user
	s.password = password
	s.database = database
	return nil
}

func (s *StdoutSink) Write(measurement string, tags map[string]string, fields map[string]interface{}, t time.Time) error {
    var tagsstr []string
    var fieldstr []string
    for k,v := range tags {
        tagsstr = append(tagsstr, fmt.Sprintf("%s=%s", k, v))
    }
    for k,v := range fields {
        if !math.IsNaN(v.(float64)) {
            fieldstr = append(fieldstr, fmt.Sprintf("%s=%v", k, v.(float64)))
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
