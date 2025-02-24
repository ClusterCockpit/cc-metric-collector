package sinks

import (
	"encoding/json"

	lp "github.com/ClusterCockpit/cc-lib/ccMessage"
	mp "github.com/ClusterCockpit/cc-lib/messageProcessor"
	influx "github.com/influxdata/line-protocol/v2/lineprotocol"
	"golang.org/x/exp/slices"
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

type key_value_pair struct {
	key   string
	value string
}

func EncoderAdd(encoder *influx.Encoder, msg lp.CCMessage) error {
	// Encode measurement name
	encoder.StartLine(msg.Name())

	tag_list := make([]key_value_pair, 0, 10)

	// copy tags and meta data which should be used as tags
	for key, value := range msg.Tags() {
		tag_list =
			append(
				tag_list,
				key_value_pair{
					key:   key,
					value: value,
				},
			)
	}
	// Encode tags (they musts be in lexical order)
	slices.SortFunc(
		tag_list,
		func(a key_value_pair, b key_value_pair) int {
			if a.key < b.key {
				return -1
			}
			if a.key > b.key {
				return +1
			}
			return 0
		},
	)
	for i := range tag_list {
		encoder.AddTag(
			tag_list[i].key,
			tag_list[i].value,
		)
	}

	// Encode fields
	for key, value := range msg.Fields() {
		encoder.AddField(key, influx.MustNewValue(value))
	}

	// Encode time stamp
	encoder.EndLine(msg.Time())

	// Return encoder errors
	return encoder.Err()
}
