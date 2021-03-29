package sinks

import (
	"bytes"
	"fmt"
	protocol "github.com/influxdata/line-protocol"
	nats "github.com/nats-io/nats.go"
	"log"
	"time"
)

type NatsSink struct {
	Sink
	client  *nats.Conn
	encoder *protocol.Encoder
	buffer  *bytes.Buffer
}

func (s *NatsSink) Init(host string, port string, user string, password string, database string) error {
	s.host = host
	s.port = port
	s.user = user
	s.password = password
	s.database = database
	// Setup Influx line protocol
	s.buffer = &bytes.Buffer{}
	s.buffer.Grow(1025)
	s.encoder = protocol.NewEncoder(s.buffer)
	s.encoder.SetPrecision(time.Second)
	s.encoder.SetMaxLineBytes(1024)
	// Setup infos for connection
	uinfo := nats.UserInfo(s.user, s.password)
	uri := fmt.Sprintf("nats://%s:%s", s.host, s.port)
	nc, err := nats.Connect(uri, uinfo)
	if err != nil {
		log.Fatal(err)
		return err
	}
	s.client = nc
	return nil
}

func (s *NatsSink) Write(measurement string, tags map[string]string, fields map[string]interface{}, t time.Time) error {
	m, err := protocol.New(measurement, tags, fields, t)
	if err != nil {
		log.Print(err)
		return err
	}
	_, err = s.encoder.Encode(m)
	if err != nil {
		log.Print(err)
		return err
	}
	s.client.Publish(s.database, s.buffer.Bytes())
	return nil
}

func (s *NatsSink) Close() {
	s.client.Close()
}
