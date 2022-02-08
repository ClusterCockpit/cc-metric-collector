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

type InfluxSinkConfig struct {
	defaultSinkConfig
	Host         string `json:"host,omitempty"`
	Port         string `json:"port,omitempty"`
	Database     string `json:"database,omitempty"`
	User         string `json:"user,omitempty"`
	Password     string `json:"password,omitempty"`
	Organization string `json:"organization,omitempty"`
	SSL          bool   `json:"ssl,omitempty"`
	RetentionPol string `json:"retention_policy,omitempty"`
}

type InfluxSink struct {
	sink
	client   influxdb2.Client
	writeApi influxdb2Api.WriteAPIBlocking
	config   InfluxSinkConfig
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
	s.client =
		influxdb2.NewClientWithOptions(
			uri,
			auth,
			influxdb2.DefaultOptions().SetTLSConfig(
				&tls.Config{InsecureSkipVerify: true},
			),
		)
	s.writeApi = s.client.WriteAPIBlocking(s.config.Organization, s.config.Database)
	return nil
}

func (s *InfluxSink) Init(config json.RawMessage) error {
	s.name = "InfluxSink"
	if len(config) > 0 {
		err := json.Unmarshal(config, &s.config)
		if err != nil {
			return err
		}
	}
	if len(s.config.Host) == 0 ||
		len(s.config.Port) == 0 ||
		len(s.config.Database) == 0 ||
		len(s.config.Organization) == 0 ||
		len(s.config.Password) == 0 {
		return errors.New("not all configuration variables set required by InfluxSink")
	}
	return s.connect()
}

func (s *InfluxSink) Write(m lp.CCMetric) error {
	err :=
		s.writeApi.WritePoint(
			context.Background(),
			m.ToPoint(s.config.MetaAsTags),
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
