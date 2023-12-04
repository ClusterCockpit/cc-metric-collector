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
	"golang.org/x/exp/slices"
)

type HttpSinkConfig struct {
	defaultSinkConfig

	// The full URL of the endpoint
	URL string `json:"url"`

	// JSON web tokens for authentication (Using the *Bearer* scheme)
	JWT string `json:"jwt,omitempty"`

	// Basic authentication
	Username     string `json:"username"`
	Password     string `json:"password"`
	useBasicAuth bool

	// time limit for requests made by the http client
	Timeout string `json:"timeout,omitempty"`
	timeout time.Duration

	// Maximum amount of time an idle (keep-alive) connection will remain idle before closing itself
	// should be larger than the measurement interval to keep the connection open
	IdleConnTimeout string `json:"idle_connection_timeout,omitempty"`
	idleConnTimeout time.Duration

	// Batch all writes arriving in during this duration
	// (default '5s', batching can be disabled by setting it to 0)
	FlushDelay string `json:"flush_delay,omitempty"`
	flushDelay time.Duration

	// Maximum number of retries to connect to the http server (default: 3)
	MaxRetries int `json:"max_retries,omitempty"`
}

type key_value_pair struct {
	key   string
	value string
}

type HttpSink struct {
	sink
	client *http.Client
	// influx line protocol encoder
	encoder influx.Encoder
	// List of tags and meta data tags which should be used as tags
	extended_tag_list []key_value_pair
	// Flush() runs in another goroutine and accesses the influx line protocol encoder,
	// so this encoderLock has to protect the encoder
	encoderLock sync.Mutex

	// timer to run Flush()
	flushTimer *time.Timer
	// Lock to assure that only one timer is running at a time
	timerLock sync.Mutex

	config HttpSinkConfig
}

// Write sends metric m as http message
func (s *HttpSink) Write(m lp.CCMetric) error {

	// Lock for encoder usage
	s.encoderLock.Lock()

	// Encode measurement name
	s.encoder.StartLine(m.Name())

	// copy tags and meta data which should be used as tags
	s.extended_tag_list = s.extended_tag_list[:0]
	for key, value := range m.Tags() {
		s.extended_tag_list =
			append(
				s.extended_tag_list,
				key_value_pair{
					key:   key,
					value: value,
				},
			)
	}
	for _, key := range s.config.MetaAsTags {
		if value, ok := m.GetMeta(key); ok {
			s.extended_tag_list =
				append(
					s.extended_tag_list,
					key_value_pair{
						key:   key,
						value: value,
					},
				)
		}
	}

	// Encode tags (they musts be in lexical order)
	slices.SortFunc(
		s.extended_tag_list,
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
	for i := range s.extended_tag_list {
		s.encoder.AddTag(
			s.extended_tag_list[i].key,
			s.extended_tag_list[i].value,
		)
	}

	// Encode fields
	for key, value := range m.Fields() {
		s.encoder.AddField(key, influx.MustNewValue(value))
	}

	// Encode time stamp
	s.encoder.EndLine(m.Time())

	// Check for encoder errors
	err := s.encoder.Err()

	// Unlock encoder usage
	s.encoderLock.Unlock()

	// Check that encoding worked
	if err != nil {
		return fmt.Errorf("Encoding failed: %v", err)
	}

	if s.config.flushDelay == 0 {

		// Directly flush if no flush delay is configured
		return s.Flush()
	} else if s.timerLock.TryLock() {

		// Setup flush timer when flush delay is configured
		// and no other timer is already running
		if s.flushTimer != nil {

			// Restarting existing flush timer
			cclog.ComponentDebug(s.name, "Write(): Restarting flush timer")
			s.flushTimer.Reset(s.config.flushDelay)
		} else {

			// Creating and starting flush timer
			cclog.ComponentDebug(s.name, "Write(): Starting new flush timer")
			s.flushTimer = time.AfterFunc(
				s.config.flushDelay,
				func() {
					defer s.timerLock.Unlock()
					cclog.ComponentDebug(s.name, "Starting flush triggered by flush timer")
					if err := s.Flush(); err != nil {
						cclog.ComponentError(s.name, "Flush triggered by flush timer: flush failed:", err)
					}
				})
		}
	}

	return nil
}

// Flush sends all metrics stored in encoder to HTTP server
func (s *HttpSink) Flush() error {

	// Lock for encoder usage
	// Own lock for as short as possible: the time it takes to clone the buffer.
	s.encoderLock.Lock()

	buf := slices.Clone(s.encoder.Bytes())
	s.encoder.Reset()

	// Unlock encoder usage
	s.encoderLock.Unlock()

	if len(buf) == 0 {
		return nil
	}

	cclog.ComponentDebug(s.name, "Flush(): Flushing")

	var res *http.Response
	for i := 0; i < s.config.MaxRetries; i++ {
		// Create new request to send buffer
		req, err := http.NewRequest(http.MethodPost, s.config.URL, bytes.NewReader(buf))
		if err != nil {
			cclog.ComponentError(s.name, "Flush(): Failed to create HTTP request:", err)
			return err
		}

		// Set authorization header
		if len(s.config.JWT) != 0 {
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.config.JWT))
		}

		// Set basic authentication
		if s.config.useBasicAuth {
			req.SetBasicAuth(s.config.Username, s.config.Password)
		}

		// Do request
		res, err = s.client.Do(req)
		if err != nil {
			cclog.ComponentError(s.name, "Flush(): transport/tcp error:", err)
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
		cclog.ComponentError(s.name, "Flush(): Application error:", err)
		return err
	}

	return nil
}

func (s *HttpSink) Close() {
	cclog.ComponentDebug(s.name, "Closing HTTP connection")

	// Stop existing timer and immediately flush
	if s.flushTimer != nil {
		if ok := s.flushTimer.Stop(); ok {
			s.timerLock.Unlock()
		}
	}

	// Flush
	if err := s.Flush(); err != nil {
		cclog.ComponentError(s.name, "Close(): Flush failed:", err)
	}

	s.client.CloseIdleConnections()
}

// NewHttpSink creates a new http sink
func NewHttpSink(name string, config json.RawMessage) (Sink, error) {
	s := new(HttpSink)
	// Set default values
	s.name = fmt.Sprintf("HttpSink(%s)", name)
	// should be larger than the measurement interval to keep the connection open
	s.config.IdleConnTimeout = "120s"
	s.config.Timeout = "5s"
	s.config.FlushDelay = "5s"
	s.config.MaxRetries = 3
	cclog.ComponentDebug(s.name, "Init()")

	// Read config
	if len(config) > 0 {
		d := json.NewDecoder(bytes.NewReader(config))
		d.DisallowUnknownFields()
		if err := d.Decode(&s.config); err != nil {
			cclog.ComponentError(s.name, "Error reading config:", err.Error())
			return nil, err
		}
	}
	if len(s.config.URL) == 0 {
		return nil, errors.New("`url` config option is required for HTTP sink")
	}

	// Check basic authentication config
	if len(s.config.Username) > 0 || len(s.config.Password) > 0 {
		s.config.useBasicAuth = true
	}
	if s.config.useBasicAuth && len(s.config.Username) == 0 {
		return nil, errors.New("basic authentication requires username")
	}
	if s.config.useBasicAuth && len(s.config.Password) == 0 {
		return nil, errors.New("basic authentication requires password")
	}

	if len(s.config.IdleConnTimeout) > 0 {
		t, err := time.ParseDuration(s.config.IdleConnTimeout)
		if err == nil {
			cclog.ComponentDebug(s.name, "Init(): idleConnTimeout", t)
			s.config.idleConnTimeout = t
		}
	}
	if len(s.config.Timeout) > 0 {
		t, err := time.ParseDuration(s.config.Timeout)
		if err == nil {
			s.config.timeout = t
			cclog.ComponentDebug(s.name, "Init(): timeout", t)
		}
	}
	if len(s.config.FlushDelay) > 0 {
		t, err := time.ParseDuration(s.config.FlushDelay)
		if err == nil {
			s.config.flushDelay = t
			cclog.ComponentDebug(s.name, "Init(): flushDelay", t)
		}
	}

	// Create http client
	s.client = &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:    1, // We will only ever talk to one host.
			IdleConnTimeout: s.config.idleConnTimeout,
		},
		Timeout: s.config.timeout,
	}

	// Configure influx line protocol encoder
	s.encoder.SetPrecision(influx.Nanosecond)
	s.extended_tag_list = make([]key_value_pair, 0)

	return s, nil
}
