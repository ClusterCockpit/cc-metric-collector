package sinks

import (
	"bytes"
	"errors"
	"fmt"
	lp "github.com/ClusterCockpit/cc-metric-collector/internal/ccMetric"
	influx "github.com/influxdata/line-protocol"
	nats "github.com/nats-io/nats.go"
	"log"
	"time"
)

type NatsSink struct {
	sink
	client  *nats.Conn
	encoder *influx.Encoder
	buffer  *bytes.Buffer
}

func (s *NatsSink) connect() error {
	uinfo := nats.UserInfo(s.user, s.password)
	uri := fmt.Sprintf("nats://%s:%s", s.host, s.port)
	log.Print("Using URI ", uri)
	s.client = nil
	nc, err := nats.Connect(uri, uinfo)
	if err != nil {
		log.Fatal(err)
		return err
	}
	s.client = nc
	return nil
}

func (s *NatsSink) Init(config sinkConfig) error {
	s.name = "NatsSink"
	if len(config.Host) == 0 ||
		len(config.Port) == 0 ||
		len(config.Database) == 0 {
		return errors.New("Not all configuration variables set required by NatsSink")
	}
	s.host = config.Host
	s.port = config.Port
	s.database = config.Database
	s.organization = config.Organization
	s.user = config.User
	s.password = config.Password
	// Setup Influx line protocol
	s.buffer = &bytes.Buffer{}
	s.buffer.Grow(1025)
	s.encoder = influx.NewEncoder(s.buffer)
	s.encoder.SetPrecision(time.Second)
	s.encoder.SetMaxLineBytes(1024)
	// Setup infos for connection
	return s.connect()
}

func (s *NatsSink) Write(point lp.CCMetric) error {
	if s.client != nil {
		_, err := s.encoder.Encode(point)
		if err != nil {
			log.Print(err)
			return err
		}
	}
	return nil
}

func (s *NatsSink) Flush() error {
	if s.client != nil {
		if err := s.client.Publish(s.database, s.buffer.Bytes()); err != nil {
			return err
		}
		s.buffer.Reset()
	}
	return nil
}

func (s *NatsSink) Close() {
	log.Print("Closing Nats connection")
	if s.client != nil {
		s.client.Close()
	}
}
