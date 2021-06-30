package sinks

import (
	//	"context"
	"crypto/tls"
	"errors"
	"fmt"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	influxdb2Api "github.com/influxdata/influxdb-client-go/v2/api"
	"log"
	"time"
)

type InfluxAsyncSink struct {
	Sink
	client    influxdb2.Client
	writeApi  influxdb2Api.WriteAPI
	retPolicy string
	errors    <-chan error
}

func (s *InfluxAsyncSink) connect() error {
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
		influxdb2.DefaultOptions().SetBatchSize(20).SetTLSConfig(&tls.Config{
			InsecureSkipVerify: true,
		}))
	s.writeApi = s.client.WriteAPI(s.organization, s.database)
	return nil
}

func (s *InfluxAsyncSink) Init(config SinkConfig) error {
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
	err := s.connect()
	s.errors = s.writeApi.Errors()
	go func() {
		for err := range s.errors {
			log.Print(err)
		}
	}()
	return err
}

func (s *InfluxAsyncSink) Write(measurement string, tags map[string]string, fields map[string]interface{}, t time.Time) error {
	p := influxdb2.NewPoint(measurement, tags, fields, t)
	s.writeApi.WritePoint(p)
	s.writeApi.Flush()
	return nil
}

func (s *InfluxAsyncSink) Close() {
	log.Print("Closing InfluxDB connection")
	s.writeApi.Flush()
	s.client.Close()
}
