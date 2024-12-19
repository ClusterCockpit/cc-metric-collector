package sinks

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	lp "github.com/ClusterCockpit/cc-energy-manager/pkg/cc-message"
	cclog "github.com/ClusterCockpit/cc-metric-collector/pkg/ccLogger"
	mp "github.com/ClusterCockpit/cc-metric-collector/pkg/messageProcessor"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	influxdb2Api "github.com/influxdata/influxdb-client-go/v2/api"
	influxdb2ApiHttp "github.com/influxdata/influxdb-client-go/v2/api/http"
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
	// Maximum number of points sent to server in single request. Default 5000
	BatchSize uint `json:"batch_size,omitempty"`
	// Interval, in ms, in which is buffer flushed if it has not been already written (by reaching batch size) . Default 1000ms
	FlushInterval         uint   `json:"flush_interval,omitempty"`
	InfluxRetryInterval   string `json:"retry_interval,omitempty"`
	InfluxExponentialBase uint   `json:"retry_exponential_base,omitempty"`
	InfluxMaxRetries      uint   `json:"max_retries,omitempty"`
	InfluxMaxRetryTime    string `json:"max_retry_time,omitempty"`
	CustomFlushInterval   string `json:"custom_flush_interval,omitempty"`
	MaxRetryAttempts      uint   `json:"max_retry_attempts,omitempty"`
	// Timestamp precision
	Precision string `json:"precision,omitempty"`
}

type InfluxAsyncSink struct {
	sink
	client              influxdb2.Client
	writeApi            influxdb2Api.WriteAPI
	errors              <-chan error
	config              InfluxAsyncSinkConfig
	influxRetryInterval uint
	influxMaxRetryTime  uint
	customFlushInterval time.Duration
	flushTimer          *time.Timer
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
		cclog.ComponentDebug(s.name, "Batch size", s.config.BatchSize)
		clientOptions.SetBatchSize(s.config.BatchSize)
	}
	if s.config.FlushInterval != 0 {
		cclog.ComponentDebug(s.name, "Flush interval", s.config.FlushInterval)
		clientOptions.SetFlushInterval(s.config.FlushInterval)
	}
	if s.influxRetryInterval != 0 {
		cclog.ComponentDebug(s.name, "MaxRetryInterval", s.influxRetryInterval)
		clientOptions.SetMaxRetryInterval(s.influxRetryInterval)
	}
	if s.influxMaxRetryTime != 0 {
		cclog.ComponentDebug(s.name, "MaxRetryTime", s.influxMaxRetryTime)
		clientOptions.SetMaxRetryTime(s.influxMaxRetryTime)
	}
	if s.config.InfluxExponentialBase != 0 {
		cclog.ComponentDebug(s.name, "Exponential Base", s.config.InfluxExponentialBase)
		clientOptions.SetExponentialBase(s.config.InfluxExponentialBase)
	}
	if s.config.InfluxMaxRetries != 0 {
		cclog.ComponentDebug(s.name, "Max Retries", s.config.InfluxMaxRetries)
		clientOptions.SetMaxRetries(s.config.InfluxMaxRetries)
	}
	clientOptions.SetTLSConfig(
		&tls.Config{
			InsecureSkipVerify: true,
		},
	)

	precision := time.Second
	if len(s.config.Precision) > 0 {
		switch s.config.Precision {
		case "s":
			precision = time.Second
		case "ms":
			precision = time.Millisecond
		case "us":
			precision = time.Microsecond
		case "ns":
			precision = time.Nanosecond
		}
	}
	clientOptions.SetPrecision(precision)

	s.client = influxdb2.NewClientWithOptions(uri, auth, clientOptions)
	s.writeApi = s.client.WriteAPI(s.config.Organization, s.config.Database)
	ok, err := s.client.Ping(context.Background())
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("connection to %s not healthy", uri)
	}
	s.writeApi.SetWriteFailedCallback(func(batch string, err influxdb2ApiHttp.Error, retryAttempts uint) bool {
		mlist := strings.Split(batch, "\n")
		cclog.ComponentError(s.name, fmt.Sprintf("Failed to write batch with %d metrics %d times (max: %d): %s", len(mlist), retryAttempts, s.config.MaxRetryAttempts, err.Error()))
		return retryAttempts <= s.config.MaxRetryAttempts
	})
	return nil
}

func (s *InfluxAsyncSink) Write(m lp.CCMessage) error {
	if s.customFlushInterval != 0 && s.flushTimer == nil {
		// Run a batched flush for all lines that have arrived in the defined interval
		s.flushTimer = time.AfterFunc(s.customFlushInterval, func() {
			if err := s.Flush(); err != nil {
				cclog.ComponentError(s.name, "flush failed:", err.Error())
			}
		})
	}
	msg, err := s.mp.ProcessMessage(m)
	if err == nil && msg != nil {
		s.writeApi.WritePoint(msg.ToPoint(nil))
	}
	return nil
}

func (s *InfluxAsyncSink) Flush() error {
	cclog.ComponentDebug(s.name, "Flushing")
	s.writeApi.Flush()
	if s.customFlushInterval != 0 && s.flushTimer != nil {
		s.flushTimer = nil
	}
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
	s.config.BatchSize = 0
	s.influxRetryInterval = 0
	//s.config.InfluxRetryInterval = "1s"
	s.influxMaxRetryTime = 0
	//s.config.InfluxMaxRetryTime = "168h"
	s.config.InfluxMaxRetries = 0
	s.config.InfluxExponentialBase = 0
	s.config.FlushInterval = 0
	s.config.CustomFlushInterval = ""
	s.customFlushInterval = time.Duration(0)
	s.config.MaxRetryAttempts = 1
	s.config.Precision = "s"

	// Default retry intervals (in seconds)
	// 1 2
	// 2 4
	// 4 8
	// 8 16
	// 16 32
	// 32 64
	// 64 128
	// 128 256
	// 256 512
	// 512 1024
	// 1024 2048
	// 2048 4096
	// 4096 8192
	// 8192 16384
	// 16384 32768
	// 32768 65536
	// 65536 131072
	// 131072 262144
	// 262144 524288

	if len(config) > 0 {
		d := json.NewDecoder(bytes.NewReader(config))
		d.DisallowUnknownFields()
		if err := d.Decode(&s.config); err != nil {
			cclog.ComponentError(s.name, "Error reading config:", err.Error())
			return nil, err
		}
	}
	if len(s.config.Port) == 0 {
		return nil, errors.New("missing port configuration required by InfluxSink")
	}
	if len(s.config.Database) == 0 {
		return nil, errors.New("missing database configuration required by InfluxSink")
	}
	if len(s.config.Organization) == 0 {
		return nil, errors.New("missing organization configuration required by InfluxSink")
	}
	if len(s.config.Password) == 0 {
		return nil, errors.New("missing password configuration required by InfluxSink")
	}
	p, err := mp.NewMessageProcessor()
	if err != nil {
		return nil, fmt.Errorf("initialization of message processor failed: %v", err.Error())
	}
	s.mp = p
	if len(s.config.MessageProcessor) > 0 {
		err = s.mp.FromConfigJSON(s.config.MessageProcessor)
		if err != nil {
			return nil, fmt.Errorf("failed parsing JSON for message processor: %v", err.Error())
		}
	}
	// Create lookup map to use meta infos as tags in the output metric
	// s.meta_as_tags = make(map[string]bool)
	// for _, k := range s.config.MetaAsTags {
	// 	s.meta_as_tags[k] = true
	// }
	for _, k := range s.config.MetaAsTags {
		s.mp.AddMoveMetaToTags("true", k, k)
	}

	toUint := func(duration string, def uint) uint {
		t, err := time.ParseDuration(duration)
		if err == nil {
			return uint(t.Milliseconds())
		}
		return def
	}
	s.influxRetryInterval = toUint(s.config.InfluxRetryInterval, s.influxRetryInterval)
	s.influxMaxRetryTime = toUint(s.config.InfluxMaxRetryTime, s.influxMaxRetryTime)

	// Use a own timer for calling Flush()
	if len(s.config.CustomFlushInterval) > 0 {
		t, err := time.ParseDuration(s.config.CustomFlushInterval)
		if err != nil {
			return nil, fmt.Errorf("invalid duration in 'custom_flush_interval': %v", err)
		}
		s.customFlushInterval = t
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
