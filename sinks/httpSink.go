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
	Host            string `json:"host,omitempty"`
	Port            string `json:"port,omitempty"`
	Database        string `json:"database,omitempty"`
	JWT             string `json:"jwt,omitempty"`
	SSL             bool   `json:"ssl,omitempty"`
	Timeout         string `json:"timeout,omitempty"`
	MaxIdleConns    int    `json:"max_idle_connections,omitempty"`
	IdleConnTimeout string `json:"idle_connection_timeout,omitempty"`
}

type HttpSink struct {
	sink
	client          *http.Client
	url, jwt        string
	encoder         *influx.Encoder
	buffer          *bytes.Buffer
	config          HttpSinkConfig
	maxIdleConns    int
	idleConnTimeout time.Duration
	timeout         time.Duration
}

func (s *HttpSink) Init(config json.RawMessage) error {
	s.name = "HttpSink"
	s.config.SSL = false
	s.config.MaxIdleConns = 10
	s.config.IdleConnTimeout = "5s"
	s.config.Timeout = "5s"
	if len(config) > 0 {
		err := json.Unmarshal(config, &s.config)
		if err != nil {
			return err
		}
	}
	if len(s.config.Host) == 0 || len(s.config.Port) == 0 || len(s.config.Database) == 0 {
		return errors.New("`host`, `port` and `database` config options required for TCP sink")
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
	proto := "http"
	if s.config.SSL {
		proto = "https"
	}
	s.url = fmt.Sprintf("%s://%s:%s/%s", proto, s.config.Host, s.config.Port, s.config.Database)
	s.jwt = s.config.JWT
	s.buffer = &bytes.Buffer{}
	s.encoder = influx.NewEncoder(s.buffer)
	s.encoder.SetPrecision(time.Second)

	return nil
}

func (s *HttpSink) Write(m lp.CCMetric) error {
	p := m.ToPoint(s.config.MetaAsTags)
	_, err := s.encoder.Encode(p)
	return err
}

func (s *HttpSink) Flush() error {
	req, err := http.NewRequest(http.MethodPost, s.url, s.buffer)
	if err != nil {
		return err
	}

	if len(s.jwt) != 0 {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.jwt))
	}

	res, err := s.client.Do(req)
	s.buffer.Reset()

	if err != nil {
		return err
	}

	if res.StatusCode != 200 {
		return errors.New(res.Status)
	}

	return nil
}

func (s *HttpSink) Close() {
	s.client.CloseIdleConnections()
}
