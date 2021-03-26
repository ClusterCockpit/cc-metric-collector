package sinks

import (
	"context"
	"fmt"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	influxdb2Api "github.com/influxdata/influxdb-client-go/v2/api"
	"log"
	"time"
)

type InfluxSink struct {
	Sink
	client       influxdb2.Client
	writeApi     influxdb2Api.WriteAPIBlocking
	retPolicy    string
	organization string
}

func (s *InfluxSink) Init(host string, port string, user string, password string, database string) error {
	s.host = host
	s.port = port
	s.user = user
	s.password = password
	s.database = database
	s.organization = ""
	uri := fmt.Sprintf("http://%s:%s", host, port)
	auth := fmt.Sprintf("%s:%s", user, password)
	log.Print("Using URI ", uri, " for connection")
	s.client = influxdb2.NewClient(uri, auth)
	s.writeApi = s.client.WriteAPIBlocking(s.organization, s.database)
	return nil
}

func (s *InfluxSink) Write(measurement string, tags map[string]string, fields map[string]interface{}, t time.Time) error {
	p := influxdb2.NewPoint(measurement, tags, fields, t)
	err := s.writeApi.WritePoint(context.Background(), p)
	return err
}

func (s *InfluxSink) Close() {
	log.Print("Closing InfluxDB connection")
	s.client.Close()
}
