package sinks

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	cclog "github.com/ClusterCockpit/cc-metric-collector/internal/ccLogger"
	lp "github.com/ClusterCockpit/cc-metric-collector/internal/ccMetric"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	influxdb2Api "github.com/influxdata/influxdb-client-go/v2/api"
)

type InfluxSinkConfig struct {
	defaultSinkConfig
	Host                  string `json:"host,omitempty"`
	Port                  string `json:"port,omitempty"`
	Database              string `json:"database,omitempty"`
	User                  string `json:"user,omitempty"`
	Password              string `json:"password,omitempty"`
	Organization          string `json:"organization,omitempty"`
	SSL                   bool   `json:"ssl,omitempty"`
	RetentionPol          string `json:"retention_policy,omitempty"`
	InfluxRetryInterval   string `json:"retry_interval"`
	InfluxExponentialBase uint   `json:"retry_exponential_base"`
	InfluxMaxRetries      uint   `json:"max_retries"`
	InfluxMaxRetryTime    string `json:"max_retry_time"`
	//InfluxMaxRetryDelay  string `json:"max_retry_delay"` // It is mentioned in the docs but there is no way to set it
}

type InfluxSink struct {
	sink
	client              influxdb2.Client
	writeApi            influxdb2Api.WriteAPIBlocking
	config              InfluxSinkConfig
	influxRetryInterval uint
	influxMaxRetryTime  uint
	//influxMaxRetryDelay uint
}

func (s *InfluxSink) connect() error {
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
	clientOptions.SetTLSConfig(
		&tls.Config{
			InsecureSkipVerify: true,
		},
	)

	clientOptions.SetMaxRetryInterval(s.influxRetryInterval)
	clientOptions.SetMaxRetryTime(s.influxMaxRetryTime)
	clientOptions.SetExponentialBase(s.config.InfluxExponentialBase)
	clientOptions.SetMaxRetries(s.config.InfluxMaxRetries)

	s.client = influxdb2.NewClientWithOptions(uri, auth, clientOptions)
	s.writeApi = s.client.WriteAPIBlocking(s.config.Organization, s.config.Database)
	ok, err := s.client.Ping(context.Background())
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("connection to %s not healthy", uri)
	}
	return nil
}

func (s *InfluxSink) Write(m lp.CCMetric) error {
	err :=
		s.writeApi.WritePoint(
			context.Background(),
			m.ToPoint(s.meta_as_tags),
		)
	return err
}

func (s *InfluxSink) Flush() error {
	return nil
}

func (s *InfluxSink) Close() {
	cclog.ComponentDebug(s.name, "Closing InfluxDB connection")
	s.client.Close()
}

func NewInfluxSink(name string, config json.RawMessage) (Sink, error) {
	s := new(InfluxSink)
	s.name = fmt.Sprintf("InfluxSink(%s)", name)
	if len(config) > 0 {
		err := json.Unmarshal(config, &s.config)
		if err != nil {
			return nil, err
		}
	}
	s.influxRetryInterval = uint(time.Duration(1) * time.Second)
	s.config.InfluxRetryInterval = "1s"
	s.influxMaxRetryTime = uint(7 * time.Duration(24) * time.Hour)
	s.config.InfluxMaxRetryTime = "168h"
	s.config.InfluxMaxRetries = 20
	s.config.InfluxExponentialBase = 2

	if len(s.config.Host) == 0 ||
		len(s.config.Port) == 0 ||
		len(s.config.Database) == 0 ||
		len(s.config.Organization) == 0 ||
		len(s.config.Password) == 0 {
		return nil, errors.New("not all configuration variables set required by InfluxSink")
	}
	// Create lookup map to use meta infos as tags in the output metric
	s.meta_as_tags = make(map[string]bool)
	for _, k := range s.config.MetaAsTags {
		s.meta_as_tags[k] = true
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

	// Connect to InfluxDB server
	if err := s.connect(); err != nil {
		return nil, fmt.Errorf("unable to connect: %v", err)
	}
	return s, nil
}
