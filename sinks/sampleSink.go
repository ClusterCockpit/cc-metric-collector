package sinks

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"

	lp "github.com/ClusterCockpit/cc-energy-manager/pkg/cc-message"
	cclog "github.com/ClusterCockpit/cc-metric-collector/pkg/ccLogger"
	mp "github.com/ClusterCockpit/cc-metric-collector/pkg/messageProcessor"
)

type SampleSinkConfig struct {
	// defines JSON tags for 'type' and 'meta_as_tags' (string list)
	// See: metricSink.go
	defaultSinkConfig
	// Additional config options, for SampleSink
}

type SampleSink struct {
	// declares elements 	'name' and 'meta_as_tags' (string to bool map!)
	sink
	config SampleSinkConfig // entry point to the SampleSinkConfig
}

// Implement functions required for Sink interface
// Write(...), Flush(), Close()
// See: metricSink.go

// Code to submit a single CCMetric to the sink
func (s *SampleSink) Write(point lp.CCMessage) error {
	// based on s.meta_as_tags use meta infos as tags
	// moreover, submit the point to the message processor
	// to apply drop/modify rules
	msg, err := s.mp.ProcessMessage(point)
	if err == nil && msg != nil {
		log.Print(msg)
	}
	return nil
}

// If the sink uses batched sends internally, you can tell to flush its buffers
func (s *SampleSink) Flush() error {
	return nil
}

// Close sink: close network connection, close files, close libraries, ...
func (s *SampleSink) Close() {
	cclog.ComponentDebug(s.name, "CLOSE")
}

// New function to create a new instance of the sink
// Initialize the sink by giving it a name and reading in the config JSON
func NewSampleSink(name string, config json.RawMessage) (Sink, error) {
	s := new(SampleSink)

	// Set name of sampleSink
	// The name should be chosen in such a way that different instances of SampleSink can be distinguished
	s.name = fmt.Sprintf("SampleSink(%s)", name) // Always specify a name here

	// Set defaults in s.config
	// Allow overwriting these defaults by reading config JSON

	// Read in the config JSON
	if len(config) > 0 {
		d := json.NewDecoder(bytes.NewReader(config))
		d.DisallowUnknownFields()
		if err := d.Decode(&s.config); err != nil {
			cclog.ComponentError(s.name, "Error reading config:", err.Error())
			return nil, err
		}
	}

	// Initialize and configure the message processor
	p, err := mp.NewMessageProcessor()
	if err != nil {
		return nil, fmt.Errorf("initialization of message processor failed: %v", err.Error())
	}
	s.mp = p

	// Add message processor configuration
	if len(s.config.MessageProcessor) > 0 {
		err = p.FromConfigJSON(s.config.MessageProcessor)
		if err != nil {
			return nil, fmt.Errorf("failed parsing JSON for message processor: %v", err.Error())
		}
	}
	// Add rules to move meta information to tag space
	// Replacing the legacy 'meta_as_tags' configuration
	for _, k := range s.config.MetaAsTags {
		s.mp.AddMoveMetaToTags("true", k, k)
	}

	// Check if all required fields in the config are set
	// E.g. use 'len(s.config.Option) > 0' for string settings

	// Establish connection to the server, library, ...
	// Check required files exist and lookup path(s) of executable(s)

	// Return (nil, meaningful error message) in case of errors
	return s, nil
}
