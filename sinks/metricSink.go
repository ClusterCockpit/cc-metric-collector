package sinks

import (
	lp "github.com/ClusterCockpit/cc-metric-collector/internal/ccMetric"
)

type defaultSinkConfig struct {
	MetaAsTags []string `json:"meta_as_tags,omitempty"`
	Type       string   `json:"type"`
}

type sink struct {
	meta_as_tags map[string]bool // Use meta data tags as tags
	name         string          // Name of the sink
}

type Sink interface {
	Write(point lp.CCMetric) error // Write metric to the sink
	Flush() error                  // Flush buffered metrics
	Close()                        // Close / finish metric sink
	Name() string                  // Name of the metric sink
}

// Name returns the name of the metric sink
func (s *sink) Name() string {
	return s.name
}
