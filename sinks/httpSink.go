package sinks

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	cclog "github.com/ClusterCockpit/cc-metric-collector/internal/ccLogger"
	lp "github.com/ClusterCockpit/cc-metric-collector/internal/ccMetric"
	influx "github.com/influxdata/line-protocol"
)

type HttpSinkConfig struct {
	defaultSinkConfig
	URL             string `json:"url,omitempty"`
	JWT             string `json:"jwt,omitempty"`
	Timeout         string `json:"timeout,omitempty"`
	IdleConnTimeout string `json:"idle_connection_timeout,omitempty"`
	FlushDelay      string `json:"flush_delay,omitempty"`
}

type HttpSink struct {
	sink
	client          *http.Client
	encoder         *influx.Encoder
	lock            sync.Mutex // Flush() runs in another goroutine, so this lock has to protect the buffer
	buffer          *bytes.Buffer
	flushTimer      *time.Timer
	config          HttpSinkConfig
	idleConnTimeout time.Duration
	timeout         time.Duration
	flushDelay      time.Duration
}

func (s *HttpSink) Write(m lp.CCMetric) error {
	if s.buffer.Len() == 0 && s.flushDelay != 0 {
		// This is the first write since the last flush, start the flushTimer!
		if s.flushTimer != nil && s.flushTimer.Stop() {
			cclog.ComponentDebug(s.name, "unexpected: the flushTimer was already running?")
		}

		// Run a batched flush for all lines that have arrived in the last second
		s.flushTimer = time.AfterFunc(s.flushDelay, func() {
			if err := s.Flush(); err != nil {
				cclog.ComponentError(s.name, "flush failed:", err.Error())
			}
		})
	}

	p := m.ToPoint(s.meta_as_tags)

	s.lock.Lock()
	_, err := s.encoder.Encode(p)
	s.lock.Unlock() // defer does not work here as Flush() takes the lock as well

	if err != nil {
		cclog.ComponentError(s.name, "encoding failed:", err.Error())
		return err
	}

	// Flush synchronously if "flush_delay" is zero
	if s.flushDelay == 0 {
		return s.Flush()
	}

	return err
}

func (s *HttpSink) Flush() error {
	// buffer is read by client.Do, prevent concurrent modifications
	s.lock.Lock()
	defer s.lock.Unlock()

	// Do not flush empty buffer
	if s.buffer.Len() == 0 {
		return nil
	}

	// Create new request to send buffer
	req, err := http.NewRequest(http.MethodPost, s.config.URL, s.buffer)
	if err != nil {
		cclog.ComponentError(s.name, "failed to create request:", err.Error())
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

	// Handle transport/tcp errors
	if err != nil {
		cclog.ComponentError(s.name, "transport/tcp error:", err.Error())
		return err
	}

	// Handle application errors
	if res.StatusCode != http.StatusOK {
		err = errors.New(res.Status)
		cclog.ComponentError(s.name, "application error:", err.Error())
		return err
	}

	return nil
}

func (s *HttpSink) Close() {
	s.flushTimer.Stop()
	if err := s.Flush(); err != nil {
		cclog.ComponentError(s.name, "flush failed:", err.Error())
	}
	s.client.CloseIdleConnections()
}

func NewHttpSink(name string, config json.RawMessage) (Sink, error) {
	s := new(HttpSink)
	// Set default values
	s.name = fmt.Sprintf("HttpSink(%s)", name)
	s.config.IdleConnTimeout = "120s" // should be larger than the measurement interval.
	s.config.Timeout = "5s"
	s.config.FlushDelay = "5s"

	// Read config
	if len(config) > 0 {
		err := json.Unmarshal(config, &s.config)
		if err != nil {
			return nil, err
		}
	}
	if len(s.config.URL) == 0 {
		return nil, errors.New("`url` config option is required for HTTP sink")
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
	if len(s.config.FlushDelay) > 0 {
		t, err := time.ParseDuration(s.config.FlushDelay)
		if err == nil {
			s.flushDelay = t
		}
	}
	// Create lookup map to use meta infos as tags in the output metric
	s.meta_as_tags = make(map[string]bool)
	for _, k := range s.config.MetaAsTags {
		s.meta_as_tags[k] = true
	}
	tr := &http.Transport{
		MaxIdleConns:    1, // We will only ever talk to one host.
		IdleConnTimeout: s.idleConnTimeout,
	}
	s.client = &http.Client{Transport: tr, Timeout: s.timeout}
	s.buffer = &bytes.Buffer{}
	s.encoder = influx.NewEncoder(s.buffer)
	s.encoder.SetPrecision(time.Second)
	return s, nil
}
