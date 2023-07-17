package sinks

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	cclog "github.com/ClusterCockpit/cc-metric-collector/pkg/ccLogger"
	lp "github.com/ClusterCockpit/cc-metric-collector/pkg/ccMetric"
	influx "github.com/influxdata/line-protocol/v2/lineprotocol"
)

type HttpSinkConfig struct {
	defaultSinkConfig
	URL             string `json:"url"`
	JWT             string `json:"jwt,omitempty"`
	Timeout         string `json:"timeout,omitempty"`
	IdleConnTimeout string `json:"idle_connection_timeout,omitempty"`
	FlushDelay      string `json:"flush_delay,omitempty"`
	MaxRetries      int    `json:"max_retries,omitempty"`
}

type HttpSink struct {
	sink
	client  *http.Client
	encoder influx.Encoder
	lock    sync.Mutex // Flush() runs in another goroutine, so this lock has to protect the buffer
	//buffer          *bytes.Buffer
	flushTimer      *time.Timer
	config          HttpSinkConfig
	idleConnTimeout time.Duration
	timeout         time.Duration
	flushDelay      time.Duration
}

func (s *HttpSink) Write(m lp.CCMetric) error {
	var err error = nil
	var firstWriteOfBatch bool = false
	p := m.ToPoint(s.meta_as_tags)
	s.lock.Lock()
	firstWriteOfBatch = len(s.encoder.Bytes()) == 0
	v, ok := m.GetField("value")
	if ok {

		s.encoder.StartLine(p.Name())
		for _, v := range p.TagList() {
			s.encoder.AddTag(v.Key, v.Value)
		}

		s.encoder.AddField("value", influx.MustNewValue(v))
		s.encoder.EndLine(p.Time())
		err = s.encoder.Err()
		if err != nil {
			cclog.ComponentError(s.name, "encoding failed:", err.Error())
			s.lock.Unlock()
			return err
		}
	}
	s.lock.Unlock()

	if s.flushDelay == 0 {
		return s.Flush()
	}

	if firstWriteOfBatch {
		if s.flushTimer == nil {
			s.flushTimer = time.AfterFunc(s.flushDelay, func() {
				if err := s.Flush(); err != nil {
					cclog.ComponentError(s.name, "flush failed:", err.Error())
				}
			})
		} else {
			s.flushTimer.Reset(s.flushDelay)
		}
	}

	return nil
}

func (s *HttpSink) Flush() error {
	// Own lock for as short as possible: the time it takes to copy the buffer.
	s.lock.Lock()
	buf := make([]byte, len(s.encoder.Bytes()))
	copy(buf, s.encoder.Bytes())
	s.encoder.Reset()
	s.lock.Unlock()
	if len(buf) == 0 {
		return nil
	}

	var res *http.Response
	for i := 0; i < s.config.MaxRetries; i++ {
		// Create new request to send buffer
		req, err := http.NewRequest(http.MethodPost, s.config.URL, bytes.NewReader(buf))
		if err != nil {
			cclog.ComponentError(s.name, "failed to create request:", err.Error())
			return err
		}

		// Set authorization header
		if len(s.config.JWT) != 0 {
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.config.JWT))
		}

		// Do request
		res, err = s.client.Do(req)
		if err != nil {
			cclog.ComponentError(s.name, "transport/tcp error:", err.Error())
			// Wait between retries
			time.Sleep(time.Duration(i+1) * (time.Second / 2))
			continue
		}

		break
	}

	if res == nil {
		return errors.New("flush failed due to repeated errors")
	}

	// Handle application errors
	if res.StatusCode != http.StatusOK {
		err := errors.New(res.Status)
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
	s.config.MaxRetries = 3
	cclog.ComponentDebug(s.name, "init")

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
			cclog.ComponentDebug(s.name, "idleConnTimeout", t)
			s.idleConnTimeout = t
		}
	}
	if len(s.config.Timeout) > 0 {
		t, err := time.ParseDuration(s.config.Timeout)
		if err == nil {
			s.timeout = t
			cclog.ComponentDebug(s.name, "timeout", t)
		}
	}
	if len(s.config.FlushDelay) > 0 {
		t, err := time.ParseDuration(s.config.FlushDelay)
		if err == nil {
			s.flushDelay = t
			cclog.ComponentDebug(s.name, "flushDelay", t)
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
	s.encoder.SetPrecision(influx.Second)
	return s, nil
}
