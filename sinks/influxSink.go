package sinks

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"log"

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
	log.Print("Using URI ", uri, " Org ", s.config.Organization, " Bucket ", s.config.Database)
	s.client = influxdb2.NewClientWithOptions(uri, auth,
		influxdb2.DefaultOptions().SetTLSConfig(&tls.Config{InsecureSkipVerify: true}))
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

func (s *InfluxSink) Write(point lp.CCMetric) error {
	tags := map[string]string{}
	fields := map[string]interface{}{}
	for key, value := range point.Tags() {
		tags[key] = value
	}
	if s.config.MetaAsTags {
		for key, value := range point.Meta() {
			tags[key] = value
		}
	}
	for _, f := range point.FieldList() {
		fields[f.Key] = f.Value
	}
	p := influxdb2.NewPoint(point.Name(), tags, fields, point.Time())
	err := s.writeApi.WritePoint(context.Background(), p)
	return err
}

func (s *InfluxSink) Flush() error {
	return nil
}

func (s *InfluxSink) Close() {
	log.Print("Closing InfluxDB connection")
	s.client.Close()
}
