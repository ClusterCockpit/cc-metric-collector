package sinks

import (
	//	"time"
	lp "github.com/influxdata/line-protocol"
)

type SinkConfig struct {
	Host         string `json:"host"`
	Port         string `json:"port"`
	Database     string `json:"database"`
	User         string `json:"user"`
	Password     string `json:"password"`
	Organization string `json:"organization"`
	Type         string `json:"type"`
	SSL          bool   `json:"ssl"`
}

type Sink struct {
	host         string
	port         string
	user         string
	password     string
	database     string
	organization string
	ssl          bool
}

type SinkFuncs interface {
	Init(config SinkConfig) error
	Write(point lp.MutableMetric) error
	Flush() error
	Close()
}
