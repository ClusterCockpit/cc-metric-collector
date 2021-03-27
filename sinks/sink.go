package sinks

import (
	"time"
)

type Sink struct {
	host     string
	port     string
	user     string
	password string
	database string
}

type SinkFuncs interface {
	Init(host string, port string, user string, password string, database string) error
	Write(measurement string, tags map[string]string, fields map[string]interface{}, t time.Time) error
	Close()
}
