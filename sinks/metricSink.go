package sinks

import (
	//	"time"
	lp "github.com/ClusterCockpit/cc-metric-collector/internal/ccMetric"
	influxlp "github.com/influxdata/line-protocol"
)

type sinkConfig struct {
	Type         string `json:"type"`
	Host         string `json:"host,omitempty"`
	Port         string `json:"port,omitempty"`
	Database     string `json:"database,omitempty"`
	User         string `json:"user,omitempty"`
	Password     string `json:"password,omitempty"`
	Organization string `json:"organization,omitempty"`
	SSL          bool   `json:"ssl,omitempty"`
	MetaAsTags   bool   `json:"meta_as_tags,omitempty"`
}

type sink struct {
	host         string
	port         string
	user         string
	password     string
	database     string
	organization string
	ssl          bool
	meta_as_tags bool
	name         string
}

type Sink interface {
	Init(config sinkConfig) error
	Write(point lp.CCMetric) error
	Flush() error
	Close()
	Name() string
}

func (s *sink) Name() string {
	return s.name
}

func Tags2Map(metric influxlp.MutableMetric) map[string]string {
	tags := make(map[string]string)
	for _, t := range metric.TagList() {
		tags[t.Key] = t.Value
	}
	return tags
}

func Fields2Map(metric influxlp.MutableMetric) map[string]interface{} {
	fields := make(map[string]interface{})
	for _, f := range metric.FieldList() {
		fields[f.Key] = f.Value
	}
	return fields
}
