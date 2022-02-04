package sinks

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"time"

	lp "github.com/ClusterCockpit/cc-metric-collector/internal/ccMetric"
	influx "github.com/influxdata/line-protocol"
)

type HttpSink struct {
	sink
	client   *http.Client
	url, jwt string
	encoder  *influx.Encoder
	buffer   *bytes.Buffer
}

func (s *HttpSink) Init(config sinkConfig) error {
	s.name = "HttpSink"
	if len(config.Host) == 0 || len(config.Port) == 0 || len(config.Database) == 0 {
		return errors.New("`host`, `port` and `database` config options required for TCP sink")
	}

	s.client = &http.Client{}
	proto := "http"
	if config.SSL {
		proto = "https"
	}
	s.url = fmt.Sprintf("%s://%s:%s/%s", proto, config.Host, config.Port, config.Database)
	s.port = config.Port
	s.jwt = config.Password
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
