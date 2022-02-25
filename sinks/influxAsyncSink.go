package sinks

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"

	cclog "github.com/ClusterCockpit/cc-metric-collector/internal/ccLogger"
	lp "github.com/ClusterCockpit/cc-metric-collector/internal/ccMetric"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	influxdb2Api "github.com/influxdata/influxdb-client-go/v2/api"
)

type InfluxAsyncSinkConfig struct {
	defaultSinkConfig
	Host         string `json:"host,omitempty"`
	Port         string `json:"port,omitempty"`
	Database     string `json:"database,omitempty"`
	User         string `json:"user,omitempty"`
	Password     string `json:"password,omitempty"`
	Organization string `json:"organization,omitempty"`
	SSL          bool   `json:"ssl,omitempty"`
	RetentionPol string `json:"retention_policy,omitempty"`
	// Maximum number of points sent to server in single request. Default 5000
	BatchSize uint `json:"batch_size,omitempty"`
	// Interval, in ms, in which is buffer flushed if it has not been already written (by reaching batch size) . Default 1000ms
	FlushInterval uint `json:"flush_interval,omitempty"`
}

type InfluxAsyncSink struct {
	sink
	client   influxdb2.Client
	writeApi influxdb2Api.WriteAPI
	errors   <-chan error
	config   InfluxAsyncSinkConfig
}

func (s *InfluxAsyncSink) connect() error {
	var auth string
	var uri string
	if s.config.SSL {
		uri = fmt.Sprintf("https://%s:%s", s.config.Host, s.config.Port)
	} else {
		uri = fmt.Sprintf("http://%s:%s", s.config.Host, s.config.Port)
	}
	if len(s.config.User) == 0 {
		auth = s.config.Password
	} else {
		auth = fmt.Sprintf("%s:%s", s.config.User, s.config.Password)
	}
	cclog.ComponentDebug(s.name, "Using URI", uri, "Org", s.config.Organization, "Bucket", s.config.Database)
	clientOptions := influxdb2.DefaultOptions()
	if s.config.BatchSize != 0 {
		clientOptions.SetBatchSize(s.config.BatchSize)
	}
	if s.config.FlushInterval != 0 {
		clientOptions.SetFlushInterval(s.config.FlushInterval)
	}
	clientOptions.SetTLSConfig(
		&tls.Config{
			InsecureSkipVerify: true,
		},
	)
	s.client = influxdb2.NewClientWithOptions(uri, auth, clientOptions)
	s.writeApi = s.client.WriteAPI(s.config.Organization, s.config.Database)
	ok, err := s.client.Ping(context.Background())
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("connection to %s not healthy", uri)
	}
	return nil
}

func (s *InfluxAsyncSink) Write(m lp.CCMetric) error {
	s.writeApi.WritePoint(
		m.ToPoint(s.config.MetaAsTags),
	)
	return nil
}

func (s *InfluxAsyncSink) Flush() error {
	s.writeApi.Flush()
	return nil
}

func (s *InfluxAsyncSink) Close() {
	cclog.ComponentDebug(s.name, "Closing InfluxDB connection")
	s.writeApi.Flush()
	s.client.Close()
}

func NewInfluxAsyncSink(name string, config json.RawMessage) (Sink, error) {
	s := new(InfluxAsyncSink)
	s.name = fmt.Sprintf("InfluxSink(%s)", name)

	// Set default for maximum number of points sent to server in single request.
	s.config.BatchSize = 100

	if len(config) > 0 {
		err := json.Unmarshal(config, &s.config)
		if err != nil {
			return nil, err
		}
	}
	if len(s.config.Host) == 0 ||
		len(s.config.Port) == 0 ||
		len(s.config.Database) == 0 ||
		len(s.config.Organization) == 0 ||
		len(s.config.Password) == 0 {
		return nil, errors.New("not all configuration variables set required by InfluxAsyncSink")
	}

	// Connect to InfluxDB server
	if err := s.connect(); err != nil {
		return nil, fmt.Errorf("unable to connect: %v", err)
	}

	// Start background: Read from error channel
	s.errors = s.writeApi.Errors()
	go func() {
		for err := range s.errors {
			cclog.ComponentError(s.name, err.Error())
		}
	}()

	return s, nil
}
