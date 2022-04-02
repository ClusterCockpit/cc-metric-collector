package sinks

import (
	"encoding/json"
	"fmt"
	"log"

	cclog "github.com/ClusterCockpit/cc-metric-collector/internal/ccLogger"
	lp "github.com/ClusterCockpit/cc-metric-collector/internal/ccMetric"
	stats "github.com/ClusterCockpit/cc-metric-collector/internal/metricRouter"
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
	// Stats counters
	statsSentMetrics int64
}

// Implement functions required for Sink interface
// Write(...), Flush(), Close()
// See: metricSink.go

// Code to submit a single CCMetric to the sink
func (s *SampleSink) Write(point lp.CCMetric) error {
	// based on s.meta_as_tags use meta infos as tags
	log.Print(point)
	s.statsSentMetrics++
	stats.ComponentStatInt(s.name, "sent_metrics", s.statsSentMetrics)
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
		err := json.Unmarshal(config, &s.config)
		if err != nil {
			return nil, err
		}
	}

	// Initalize stats counters
	s.statsSentMetrics = 0

	// Create lookup map to use meta infos as tags in the output metric
	s.meta_as_tags = make(map[string]bool)
	for _, k := range s.config.MetaAsTags {
		s.meta_as_tags[k] = true
	}

	// Check if all required fields in the config are set
	// E.g. use 'len(s.config.Option) > 0' for string settings

	// Establish connection to the server, library, ...
	// Check required files exist and lookup path(s) of executable(s)

	// Return (nil, meaningful error message) in case of errors
	return s, nil
}
