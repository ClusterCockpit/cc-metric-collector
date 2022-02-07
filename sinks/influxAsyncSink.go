package sinks

import (
	//	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"

	cclog "github.com/ClusterCockpit/cc-metric-collector/internal/ccLogger"
	lp "github.com/ClusterCockpit/cc-metric-collector/internal/ccMetric"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	influxdb2Api "github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/influxdata/influxdb-client-go/v2/api/write"
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
	RetentionPol string `json:"retention_policy,omitempty"`
}

type InfluxAsyncSink struct {
	sink
	client    influxdb2.Client
	writeApi  influxdb2Api.WriteAPI
	retPolicy string
	errors    <-chan error
	config    InfluxAsyncSinkConfig
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
	s.client = influxdb2.NewClientWithOptions(uri, auth,
		influxdb2.DefaultOptions().SetBatchSize(20).SetTLSConfig(&tls.Config{
			InsecureSkipVerify: true,
		}))
	s.writeApi = s.client.WriteAPI(s.config.Organization, s.config.Database)
	return nil
}

func (s *InfluxAsyncSink) Init(config json.RawMessage) error {
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
		return errors.New("not all configuration variables set required by InfluxAsyncSink")
	}
	err := s.connect()
	s.errors = s.writeApi.Errors()
	go func() {
		for err := range s.errors {
			cclog.ComponentError(s.name, err.Error())
		}
	}()
	return err
}

func (s *InfluxAsyncSink) Write(point lp.CCMetric) error {
	var p *write.Point
	if s.config.MetaAsTags {
		tags := map[string]string{}
		for k, v := range point.Tags() {
			tags[k] = v
		}
		for k, v := range point.Meta() {
			tags[k] = v
		}
		p = influxdb2.NewPoint(point.Name(), tags, point.Fields(), point.Time())
	} else {
		p = influxdb2.NewPoint(point.Name(), point.Tags(), point.Fields(), point.Time())
	}

	s.writeApi.WritePoint(p)
	return nil
}

func (s *InfluxAsyncSink) Flush() error {
	s.writeApi.Flush()
	return nil
}

func (s *InfluxAsyncSink) Close() {
	cclog.ComponentDebug(s.name, "Closing InfluxDB connection")
	s.writeApi.Flush()
	s.client.Close()
}
