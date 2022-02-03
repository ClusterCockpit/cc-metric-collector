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
	Host     string `json:"host,omitempty"`
	Port     string `json:"port,omitempty"`
	Database string `json:"database,omitempty"`
	JWT      string `json:"jwt,omitempty"`
}

type HttpSink struct {
	sink
	client   *http.Client
	url, jwt string
	encoder  *influx.Encoder
	buffer   *bytes.Buffer
	config   HttpSinkConfig
}

func (s *HttpSink) Init(config json.RawMessage) error {
	s.name = "HttpSink"
	if len(config) > 0 {
		err := json.Unmarshal(config, &s.config)
		if err != nil {
			return err
		}
	}
	if len(s.config.Host) == 0 || len(s.config.Port) == 0 || len(s.config.Database) == 0 {
		return errors.New("`host`, `port` and `database` config options required for TCP sink")
	}

	s.client = &http.Client{}
	s.url = fmt.Sprintf("http://%s:%s/%s", s.config.Host, s.config.Port, s.config.Database)
	s.jwt = s.config.JWT
	s.buffer = &bytes.Buffer{}
	s.encoder = influx.NewEncoder(s.buffer)
	s.encoder.SetPrecision(time.Second)

	return nil
}

func (s *HttpSink) Write(point lp.CCMetric) error {
	_, err := s.encoder.Encode(point)
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
