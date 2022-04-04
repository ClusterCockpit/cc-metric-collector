package sinks

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	cclog "github.com/ClusterCockpit/cc-metric-collector/internal/ccLogger"
	lp "github.com/ClusterCockpit/cc-metric-collector/internal/ccMetric"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	influxdb2Api "github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/influxdata/influxdb-client-go/v2/api/write"
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
	FlushDelay   string `json:"flush_delay,omitempty"`
	BatchSize    int    `json:"batch_size,omitempty"`
	RetentionPol string `json:"retention_policy,omitempty"`
	// InfluxRetryInterval   string `json:"retry_interval"`
	// InfluxExponentialBase uint   `json:"retry_exponential_base"`
	// InfluxMaxRetries      uint   `json:"max_retries"`
	// InfluxMaxRetryTime    string `json:"max_retry_time"`
	//InfluxMaxRetryDelay  string `json:"max_retry_delay"` // It is mentioned in the docs but there is no way to set it
}

type InfluxSink struct {
	sink
	client              influxdb2.Client
	writeApi            influxdb2Api.WriteAPIBlocking
	config              InfluxSinkConfig
	influxRetryInterval uint
	influxMaxRetryTime  uint
	batch               []*write.Point
	flushTimer          *time.Timer
	flushDelay          time.Duration
	lock                sync.Mutex // Flush() runs in another goroutine, so this lock has to protect the buffer
	//influxMaxRetryDelay uint
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
	cclog.ComponentDebug(s.name, "Using URI", uri, "Org", s.config.Organization, "Bucket", s.config.Database)
	clientOptions := influxdb2.DefaultOptions()

	// if s.influxRetryInterval != 0 {
	// 	cclog.ComponentDebug(s.name, "MaxRetryInterval", s.influxRetryInterval)
	// 	clientOptions.SetMaxRetryInterval(s.influxRetryInterval)
	// }
	// if s.influxMaxRetryTime != 0 {
	// 	cclog.ComponentDebug(s.name, "MaxRetryTime", s.influxMaxRetryTime)
	// 	clientOptions.SetMaxRetryTime(s.influxMaxRetryTime)
	// }
	// if s.config.InfluxExponentialBase != 0 {
	// 	cclog.ComponentDebug(s.name, "Exponential Base", s.config.InfluxExponentialBase)
	// 	clientOptions.SetExponentialBase(s.config.InfluxExponentialBase)
	// }
	// if s.config.InfluxMaxRetries != 0 {
	// 	cclog.ComponentDebug(s.name, "Max Retries", s.config.InfluxMaxRetries)
	// 	clientOptions.SetMaxRetries(s.config.InfluxMaxRetries)
	// }

	clientOptions.SetTLSConfig(
		&tls.Config{
			InsecureSkipVerify: true,
		},
	)

	clientOptions.SetPrecision(time.Second)

	s.client = influxdb2.NewClientWithOptions(uri, auth, clientOptions)
	s.writeApi = s.client.WriteAPIBlocking(s.config.Organization, s.config.Database)
	ok, err := s.client.Ping(context.Background())
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("connection to %s not healthy", uri)
	}
	return nil
}

func (s *InfluxSink) Write(m lp.CCMetric) error {
	// err :=
	// 	s.writeApi.WritePoint(
	// 		context.Background(),
	// 		m.ToPoint(s.meta_as_tags),
	// 	)
	if len(s.batch) == 0 && s.flushDelay != 0 {
		// This is the first write since the last flush, start the flushTimer!
		if s.flushTimer != nil && s.flushTimer.Stop() {
			cclog.ComponentDebug(s.name, "unexpected: the flushTimer was already running?")
		}

		// Run a batched flush for all lines that have arrived in the last second
		s.flushTimer = time.AfterFunc(s.flushDelay, func() {
			if err := s.Flush(); err != nil {
				cclog.ComponentError(s.name, "flush failed:", err.Error())
			}
		})
	}
	p := m.ToPoint(s.meta_as_tags)
	s.lock.Lock()
	s.batch = append(s.batch, p)
	s.lock.Unlock()

	// Flush synchronously if "flush_delay" is zero
	if s.flushDelay == 0 {
		return s.Flush()
	}

	return nil
}

func (s *InfluxSink) Flush() error {
	s.lock.Lock()
	defer s.lock.Unlock()
	if len(s.batch) == 0 {
		return nil
	}
	err := s.writeApi.WritePoint(context.Background(), s.batch...)
	if err != nil {
		cclog.ComponentError(s.name, "flush failed:", err.Error())
		return err
	}
	s.batch = s.batch[:0]
	return nil
}

func (s *InfluxSink) Close() {
	cclog.ComponentDebug(s.name, "Closing InfluxDB connection")
	s.flushTimer.Stop()
	s.Flush()
	s.client.Close()
}

func NewInfluxSink(name string, config json.RawMessage) (Sink, error) {
	s := new(InfluxSink)
	s.name = fmt.Sprintf("InfluxSink(%s)", name)
	s.config.BatchSize = 100
	s.config.FlushDelay = "1s"
	if len(config) > 0 {
		err := json.Unmarshal(config, &s.config)
		if err != nil {
			return nil, err
		}
	}
	s.influxRetryInterval = 0
	s.influxMaxRetryTime = 0
	// s.config.InfluxRetryInterval = ""
	// s.config.InfluxMaxRetryTime = ""
	// s.config.InfluxMaxRetries = 0
	// s.config.InfluxExponentialBase = 0

	if len(s.config.Host) == 0 ||
		len(s.config.Port) == 0 ||
		len(s.config.Database) == 0 ||
		len(s.config.Organization) == 0 ||
		len(s.config.Password) == 0 {
		return nil, errors.New("not all configuration variables set required by InfluxSink")
	}
	// Create lookup map to use meta infos as tags in the output metric
	s.meta_as_tags = make(map[string]bool)
	for _, k := range s.config.MetaAsTags {
		s.meta_as_tags[k] = true
	}

	// toUint := func(duration string, def uint) uint {
	// 	if len(duration) > 0 {
	// 		t, err := time.ParseDuration(duration)
	// 		if err == nil {
	// 			return uint(t.Milliseconds())
	// 		}
	// 	}
	// 	return def
	// }
	// s.influxRetryInterval = toUint(s.config.InfluxRetryInterval, s.influxRetryInterval)
	// s.influxMaxRetryTime = toUint(s.config.InfluxMaxRetryTime, s.influxMaxRetryTime)

	if len(s.config.FlushDelay) > 0 {
		t, err := time.ParseDuration(s.config.FlushDelay)
		if err == nil {
			s.flushDelay = t
		}
	}
	s.batch = make([]*write.Point, 0, s.config.BatchSize)

	// Connect to InfluxDB server
	if err := s.connect(); err != nil {
		return nil, fmt.Errorf("unable to connect: %v", err)
	}
	return s, nil
}
