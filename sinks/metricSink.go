package sinks

import (
	"encoding/json"

	lp "github.com/ClusterCockpit/cc-energy-manager/pkg/cc-message"
	mp "github.com/ClusterCockpit/cc-metric-collector/pkg/messageProcessor"
)

type defaultSinkConfig struct {
	MetaAsTags       []string        `json:"meta_as_tags,omitempty"`
	MessageProcessor json.RawMessage `json:"process_messages,omitempty"`
	Type             string          `json:"type"`
}

type sink struct {
	meta_as_tags map[string]bool     // Use meta data tags as tags
	mp           mp.MessageProcessor // message processor for the sink
	name         string              // Name of the sink
}

type Sink interface {
	Write(point lp.CCMessage) error // Write metric to the sink
	Flush() error                   // Flush buffered metrics
	Close()                         // Close / finish metric sink
	Name() string                   // Name of the metric sink
}

// Name returns the name of the metric sink
func (s *sink) Name() string {
	return s.name
}
