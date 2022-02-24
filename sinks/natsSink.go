package sinks

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	cclog "github.com/ClusterCockpit/cc-metric-collector/internal/ccLogger"
	lp "github.com/ClusterCockpit/cc-metric-collector/internal/ccMetric"
	influx "github.com/influxdata/line-protocol"
	nats "github.com/nats-io/nats.go"
)

type NatsSinkConfig struct {
	defaultSinkConfig
	Host     string `json:"host,omitempty"`
	Port     string `json:"port,omitempty"`
	Database string `json:"database,omitempty"`
	User     string `json:"user,omitempty"`
	Password string `json:"password,omitempty"`
}

type NatsSink struct {
	sink
	client  *nats.Conn
	encoder *influx.Encoder
	buffer  *bytes.Buffer
	config  NatsSinkConfig
}

func (s *NatsSink) connect() error {
	var err error
	var uinfo nats.Option = nil
	var nc *nats.Conn
	if len(s.config.User) > 0 && len(s.config.Password) > 0 {
		uinfo = nats.UserInfo(s.config.User, s.config.Password)
	}
	uri := fmt.Sprintf("nats://%s:%s", s.config.Host, s.config.Port)
	cclog.ComponentDebug(s.name, "Connect to", uri)
	s.client = nil
	if uinfo != nil {
		nc, err = nats.Connect(uri, uinfo)
	} else {
		nc, err = nats.Connect(uri)
	}
	if err != nil {
		cclog.ComponentError(s.name, "Connect to", uri, "failed:", err.Error())
		return err
	}
	s.client = nc
	return nil
}

func (s *NatsSink) Write(m lp.CCMetric) error {
	if s.client != nil {
		_, err := s.encoder.Encode(m.ToPoint(s.config.MetaAsTags))
		if err != nil {
			cclog.ComponentError(s.name, "Write:", err.Error())
			return err
		}
	}
	return nil
}

func (s *NatsSink) Flush() error {
	if s.client != nil {
		if err := s.client.Publish(s.config.Database, s.buffer.Bytes()); err != nil {
			cclog.ComponentError(s.name, "Flush:", err.Error())
			return err
		}
		s.buffer.Reset()
	}
	return nil
}

func (s *NatsSink) Close() {
	if s.client != nil {
		cclog.ComponentDebug(s.name, "Close")
		s.client.Close()
	}
}

func NewNatsSink(name string, config json.RawMessage) (Sink, error) {
	s := new(NatsSink)
	s.name = fmt.Sprintf("NatsSink(%s)", name)
	if len(config) > 0 {
		err := json.Unmarshal(config, &s.config)
		if err != nil {
			cclog.ComponentError(s.name, "Error reading config for", s.name, ":", err.Error())
			return nil, err
		}
	}
	if len(s.config.Host) == 0 ||
		len(s.config.Port) == 0 ||
		len(s.config.Database) == 0 {
		return nil, errors.New("not all configuration variables set required by NatsSink")
	}
	// Setup Influx line protocol
	s.buffer = &bytes.Buffer{}
	s.buffer.Grow(1025)
	s.encoder = influx.NewEncoder(s.buffer)
	s.encoder.SetPrecision(time.Second)
	s.encoder.SetMaxLineBytes(1024)
	// Setup infos for connection
	if err := s.connect(); err != nil {
		return nil, fmt.Errorf("Unable to connect: %v", err)
	}
	return s, nil
}
