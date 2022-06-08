package sinks

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	cclog "github.com/ClusterCockpit/cc-metric-collector/internal/ccLogger"
	lp "github.com/ClusterCockpit/cc-metric-collector/internal/ccMetric"
	influx "github.com/influxdata/line-protocol"
	nats "github.com/nats-io/nats.go"
)

type NatsSinkConfig struct {
	defaultSinkConfig
	Host       string `json:"host,omitempty"`
	Port       string `json:"port,omitempty"`
	Subject    string `json:"subject,omitempty"`
	User       string `json:"user,omitempty"`
	Password   string `json:"password,omitempty"`
	FlushDelay string `json:"flush_delay,omitempty"`
}

type NatsSink struct {
	sink
	client  *nats.Conn
	encoder *influx.Encoder
	buffer  *bytes.Buffer
	config  NatsSinkConfig

	lock       sync.Mutex
	flushDelay time.Duration
	flushTimer *time.Timer
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
	s.lock.Lock()
	_, err := s.encoder.Encode(m.ToPoint(s.meta_as_tags))
	s.lock.Unlock()
	if err != nil {
		cclog.ComponentError(s.name, "Write:", err.Error())
		return err
	}

	if s.flushDelay == 0 {
		s.Flush()
	} else if s.flushTimer == nil {
		s.flushTimer = time.AfterFunc(s.flushDelay, func() {
			s.Flush()
		})
	} else {
		s.flushTimer.Reset(s.flushDelay)
	}

	return nil
}

func (s *NatsSink) Flush() error {
	s.lock.Lock()
	buf := append([]byte{}, s.buffer.Bytes()...) // copy bytes
	s.buffer.Reset()
	s.lock.Unlock()

	if len(buf) == 0 {
		return nil
	}

	if err := s.client.Publish(s.config.Subject, buf); err != nil {
		cclog.ComponentError(s.name, "Flush:", err.Error())
		return err
	}
	return nil
}

func (s *NatsSink) Close() {
	cclog.ComponentDebug(s.name, "Close")
	s.client.Close()
}

func NewNatsSink(name string, config json.RawMessage) (Sink, error) {
	s := new(NatsSink)
	s.name = fmt.Sprintf("NatsSink(%s)", name)
	s.flushDelay = 10 * time.Second
	if len(config) > 0 {
		err := json.Unmarshal(config, &s.config)
		if err != nil {
			cclog.ComponentError(s.name, "Error reading config for", s.name, ":", err.Error())
			return nil, err
		}
	}
	if len(s.config.Host) == 0 ||
		len(s.config.Port) == 0 ||
		len(s.config.Subject) == 0 {
		return nil, errors.New("not all configuration variables set required by NatsSink")
	}
	// Create lookup map to use meta infos as tags in the output metric
	s.meta_as_tags = make(map[string]bool)
	for _, k := range s.config.MetaAsTags {
		s.meta_as_tags[k] = true
	}
	// Setup Influx line protocol
	s.buffer = &bytes.Buffer{}
	s.buffer.Grow(1025)
	s.encoder = influx.NewEncoder(s.buffer)
	s.encoder.SetPrecision(time.Second)
	s.encoder.SetMaxLineBytes(1024)
	// Setup infos for connection
	if err := s.connect(); err != nil {
		return nil, fmt.Errorf("unable to connect: %v", err)
	}

	s.flushTimer = nil
	if len(s.config.FlushDelay) != 0 {
		var err error
		s.flushDelay, err = time.ParseDuration(s.config.FlushDelay)
		if err != nil {
			return nil, err
		}
	}

	return s, nil
}
