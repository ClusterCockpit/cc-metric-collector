# CCMetric sinks

This folder contains the SinkManager and sink implementations for the cc-metric-collector.

# Available sinks:
- [`stdout`](./stdoutSink.md): Print all metrics to `stdout`, `stderr` or a file
- [`http`](./httpSink.md): Send metrics to an HTTP server as POST requests
- [`influxdb`](./influxSink.md): Send metrics to an [InfluxDB](https://www.influxdata.com/products/influxdb/) database
- [`influxasync`](./influxAsyncSink.md): Send metrics to an [InfluxDB](https://www.influxdata.com/products/influxdb/) database with non-blocking write API
- [`nats`](./natsSink.md): Publish metrics to the [NATS](https://nats.io/) network overlay system
- [`ganglia`](./gangliaSink.md): Publish metrics in the [Ganglia Monitoring System](http://ganglia.info/) using the `gmetric` CLI tool
- [`libganglia`](./libgangliaSink.md): Publish metrics in the [Ganglia Monitoring System](http://ganglia.info/) directly using `libganglia.so`
- [`prometeus`](./prometheusSink.md): Publish metrics for the [Prometheus Monitoring System](https://prometheus.io/)

# Configuration

The configuration file for the sinks is a list of configurations. The `type` field in each specifies which sink to initialize.

```json
{
  "mystdout" : {
    "type" : "stdout",
    "meta_as_tags" : [
    	"unit"
    ]
  },
  "metricstore" : {
    "type" : "http",
    "host" : "localhost",
    "port" : "4123",
    "database" : "ccmetric",
    "password" : "<jwt token>"
  }
}
```




# Contributing own sinks
A sink contains five functions and is derived from the type `sink`:
* `Init(name string, config json.RawMessage) error`
* `Write(point CCMetric) error`
* `Flush() error`
* `Close()`
* `New<Typename>(name string, config json.RawMessage) (Sink, error)` (calls the `Init()` function)

The data structures should be set up in `Init()` like opening a file or server connection. The `Write()` function writes/sends the data. For non-blocking sinks, the `Flush()` method tells the sink to drain its internal buffers. The `Close()` function should tear down anything created in `Init()`.

Finally, the sink needs to be registered in the `sinkManager.go`. There is a list of sinks called `AvailableSinks` which is a map (`sink_type_string` -> `pointer to sink interface`). Add a new entry with a descriptive name and the new sink.

## Sample sink

```go
package sinks

import (
	"encoding/json"
	"log"
	lp "github.com/ClusterCockpit/cc-metric-collector/internal/ccMetric"
)

type SampleSinkConfig struct {
	defaultSinkConfig  // defines JSON tags for 'name' and 'meta_as_tags'
}

type SampleSink struct {
	sink              // declarate 'name' and 'meta_as_tags'
	config StdoutSinkConfig // entry point to the SampleSinkConfig
}

// Initialize the sink by giving it a name and reading in the config JSON
func (s *SampleSink) Init(name string, config json.RawMessage) error {
	s.name = fmt.Sprintf("SampleSink(%s)", name)   // Always specify a name here
  // Read in the config JSON
	if len(config) > 0 {
		err := json.Unmarshal(config, &s.config)
		if err != nil {
			return err
		}
	}
	return nil
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
func (s *SampleSink) Close() {}


// New function to create a new instance of the sink
func NewSampleSink(name string, config json.RawMessage) (Sink, error) {
	s := new(SampleSink)
	err := s.Init(name, config)
	return s, err
}

```
