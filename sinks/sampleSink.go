package sinks

import (
	"encoding/json"
	"fmt"
	"log"

	cclog "github.com/ClusterCockpit/cc-metric-collector/internal/ccLogger"
	lp "github.com/ClusterCockpit/cc-metric-collector/internal/ccMetric"
)

type SampleSinkConfig struct {
	defaultSinkConfig // defines JSON tags for 'type' and 'meta_as_tags'
	// Add additional options
}

type SampleSink struct {
	sink                    // declarate 'name' and 'meta_as_tags'
	config SampleSinkConfig // entry point to the SampleSinkConfig
}

// Code to submit a single CCMetric to the sink
func (s *SampleSink) Write(point lp.CCMetric) error {
	log.Print(point)
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
	s.name = fmt.Sprintf("SampleSink(%s)", name) // Always specify a name here

	// Set defaults in s.config

	// Read in the config JSON
	if len(config) > 0 {
		err := json.Unmarshal(config, &s.config)
		if err != nil {
			return nil, err
		}
	}

	// Check if all required fields in the config are set
	// Use 'len(s.config.Option) > 0' for string settings

	// Establish connection to the server, library, ...
	// Check required files exist and lookup path(s) of executable(s)

	// Return (nil, meaningful error message) in case of errors
	return s, nil
}
