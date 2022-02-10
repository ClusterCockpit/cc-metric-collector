package sinks

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	lp "github.com/ClusterCockpit/cc-metric-collector/internal/ccMetric"
	influx "github.com/influxdata/line-protocol"
)

type HttpSinkConfig struct {
	defaultSinkConfig
	URL             string `json:"url,omitempty"`
	JWT             string `json:"jwt,omitempty"`
	Timeout         string `json:"timeout,omitempty"`
	MaxIdleConns    int    `json:"max_idle_connections,omitempty"`
	IdleConnTimeout string `json:"idle_connection_timeout,omitempty"`
	BatchSize       int    `json:"batch_size,omitempty"`
}

type HttpSink struct {
	sink
	client          *http.Client
	encoder         *influx.Encoder
	buffer          *bytes.Buffer
	config          HttpSinkConfig
	maxIdleConns    int
	idleConnTimeout time.Duration
	timeout         time.Duration
	batchCounter    int
}

func (s *HttpSink) Init(config json.RawMessage) error {
	// Set default values
	s.name = "HttpSink"
	s.config.MaxIdleConns = 10
	s.config.IdleConnTimeout = "5s"
	s.config.Timeout = "5s"
	s.config.BatchSize = 20

	// Reset counter
	s.batchCounter = 0

	// Read config
	if len(config) > 0 {
		err := json.Unmarshal(config, &s.config)
		if err != nil {
			return err
		}
	}
	if len(s.config.URL) == 0 {
		return errors.New("`url` config option is required for HTTP sink")
	}
	if s.config.MaxIdleConns > 0 {
		s.maxIdleConns = s.config.MaxIdleConns
	}
	if len(s.config.IdleConnTimeout) > 0 {
		t, err := time.ParseDuration(s.config.IdleConnTimeout)
		if err == nil {
			s.idleConnTimeout = t
		}
	}
	if len(s.config.Timeout) > 0 {
		t, err := time.ParseDuration(s.config.Timeout)
		if err == nil {
			s.timeout = t
		}
	}
	tr := &http.Transport{
		MaxIdleConns:    s.maxIdleConns,
		IdleConnTimeout: s.idleConnTimeout,
	}
	s.client = &http.Client{Transport: tr, Timeout: s.timeout}
	s.buffer = &bytes.Buffer{}
	s.encoder = influx.NewEncoder(s.buffer)
	s.encoder.SetPrecision(time.Second)

	return nil
}

func (s *HttpSink) Write(m lp.CCMetric) error {
	p := m.ToPoint(s.config.MetaAsTags)
	_, err := s.encoder.Encode(p)

	// Flush when received more metrics than batch size
	s.batchCounter++
	if s.batchCounter > s.config.BatchSize {
		s.Flush()
	}
	return err
}

func (s *HttpSink) Flush() error {
	// Do not flush empty buffer
	if s.batchCounter == 0 {
		return nil
	}

	// Reset counter
	s.batchCounter = 0

	// Create new request to send buffer
	req, err := http.NewRequest(http.MethodPost, s.config.URL, s.buffer)
	if err != nil {
		return err
	}

	// Set authorization header
	if len(s.config.JWT) != 0 {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.config.JWT))
	}

	// Send
	res, err := s.client.Do(req)

	// Clear buffer
	s.buffer.Reset()

	// Handle error code
	if err != nil {
		return err
	}

	// Handle status code
	if res.StatusCode != http.StatusOK {
		return errors.New(res.Status)
	}

	return nil
}

func (s *HttpSink) Close() {
	s.Flush()
	s.client.CloseIdleConnections()
}
