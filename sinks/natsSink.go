package sinks

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	lp "github.com/ClusterCockpit/cc-energy-manager/pkg/cc-message"
	cclog "github.com/ClusterCockpit/cc-metric-collector/pkg/ccLogger"
	mp "github.com/ClusterCockpit/cc-metric-collector/pkg/messageProcessor"
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
	NkeyFile   string `json:"nkey_file,omitempty"`
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
	} else if len(s.config.NkeyFile) > 0 {
		if _, err := os.Stat(s.config.NkeyFile); err == nil {
			uinfo = nats.UserCredentials(s.config.NkeyFile)
		} else {
			cclog.ComponentError(s.name, "NKEY file", s.config.NkeyFile, "does not exist: %v", err.Error())
			return err
		}
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

func (s *NatsSink) Write(m lp.CCMessage) error {
	msg, err := s.mp.ProcessMessage(m)
	if err == nil && msg != nil {
		s.lock.Lock()
		_, err := s.encoder.Encode(msg.ToPoint(nil))
		s.lock.Unlock()
		if err != nil {
			cclog.ComponentError(s.name, "Write:", err.Error())
			return err
		}
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
	s.config.Port = "4222"
	if len(config) > 0 {
		d := json.NewDecoder(bytes.NewReader(config))
		d.DisallowUnknownFields()
		if err := d.Decode(&s.config); err != nil {
			cclog.ComponentError(s.name, "Error reading config:", err.Error())
			return nil, err
		}
	}
	if len(s.config.Host) == 0 ||
		len(s.config.Port) == 0 ||
		len(s.config.Subject) == 0 {
		return nil, errors.New("not all configuration variables set required by NatsSink")
	}
	p, err := mp.NewMessageProcessor()
	if err != nil {
		return nil, fmt.Errorf("initialization of message processor failed: %v", err.Error())
	}
	s.mp = p
	if len(s.config.MessageProcessor) > 0 {
		err = s.mp.FromConfigJSON(s.config.MessageProcessor)
		if err != nil {
			return nil, fmt.Errorf("failed parsing JSON for message processor: %v", err.Error())
		}
	}
	// Create lookup map to use meta infos as tags in the output metric
	for _, k := range s.config.MetaAsTags {
		s.mp.AddMoveMetaToTags("true", k, k)
	}
	// s.meta_as_tags = make(map[string]bool)
	// for _, k := range s.config.MetaAsTags {
	// 	s.meta_as_tags[k] = true
	// }
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
