package sinks

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log"

	lp "github.com/ClusterCockpit/cc-metric-collector/internal/ccMetric"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	influxdb2Api "github.com/influxdata/influxdb-client-go/v2/api"
)

type InfluxSink struct {
	sink
	client    influxdb2.Client
	writeApi  influxdb2Api.WriteAPIBlocking
	retPolicy string
}

func (s *InfluxSink) connect() error {
	var auth string
	var uri string
	if s.ssl {
		uri = fmt.Sprintf("https://%s:%s", s.host, s.port)
	} else {
		uri = fmt.Sprintf("http://%s:%s", s.host, s.port)
	}
	if len(s.user) == 0 {
		auth = s.password
	} else {
		auth = fmt.Sprintf("%s:%s", s.user, s.password)
	}
	log.Print("Using URI ", uri, " Org ", s.organization, " Bucket ", s.database)
	s.client = influxdb2.NewClientWithOptions(uri, auth,
		influxdb2.DefaultOptions().SetTLSConfig(&tls.Config{InsecureSkipVerify: true}))
	s.writeApi = s.client.WriteAPIBlocking(s.organization, s.database)
	return nil
}

func (s *InfluxSink) Init(config sinkConfig) error {
	s.name = "InfluxSink"
	if len(config.Host) == 0 ||
		len(config.Port) == 0 ||
		len(config.Database) == 0 ||
		len(config.Organization) == 0 ||
		len(config.Password) == 0 {
		return errors.New("Not all configuration variables set required by InfluxSink")
	}
	s.host = config.Host
	s.port = config.Port
	s.database = config.Database
	s.organization = config.Organization
	s.user = config.User
	s.password = config.Password
	s.ssl = config.SSL
	s.meta_as_tags = config.MetaAsTags
	return s.connect()
}

func (s *InfluxSink) Write(point lp.CCMetric) error {
	tags := map[string]string{}
	fields := map[string]interface{}{}
	for key, value := range point.TagMap() {
		tags[key] = value
	}
	if s.meta_as_tags {
		for key, value := range point.MetaMap() {
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
