package sinks

import (
	"encoding/json"

	lp "github.com/ClusterCockpit/cc-metric-collector/internal/ccMetric"
)

type defaultSinkConfig struct {
	MetaAsTags bool   `json:"meta_as_tags,omitempty"`
	Type       string `json:"type"`
}

type sink struct {
	meta_as_tags bool
	name         string
}

type Sink interface {
	Init(config json.RawMessage) error
	Write(point lp.CCMetric) error
	Flush() error
	Close()
	Name() string
}

func (s *sink) Name() string {
	return s.name
}
