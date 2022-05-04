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

// connect connects to the InfluxDB server
func (s *InfluxSink) connect() error {

	// URI options:
	// * http://host:port
	// * https://host:port
	var uri string
	if s.config.SSL {
		uri = fmt.Sprintf("https://%s:%s", s.config.Host, s.config.Port)
	} else {
		uri = fmt.Sprintf("http://%s:%s", s.config.Host, s.config.Port)
	}

	// Authentication options:
	// * token
	// * username:password
	var auth string
	if len(s.config.User) == 0 {
		auth = s.config.Password
	} else {
		auth = fmt.Sprintf("%s:%s", s.config.User, s.config.Password)
	}
	cclog.ComponentDebug(s.name, "Using URI", uri, "Org", s.config.Organization, "Bucket", s.config.Database)

	// Set influxDB client options
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

	// Do not check InfluxDB certificate
	clientOptions.SetTLSConfig(
		&tls.Config{
			InsecureSkipVerify: true,
		},
	)

	clientOptions.SetPrecision(time.Second)

	// Create new writeAPI
	s.client = influxdb2.NewClientWithOptions(uri, auth, clientOptions)
	s.writeApi = s.client.WriteAPIBlocking(s.config.Organization, s.config.Database)

	// Check InfluxDB server accessibility
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

	if len(s.batch) == 0 && s.flushDelay != 0 {
		// This is the first write since the last flush, start the flushTimer!
		if s.flushTimer != nil && s.flushTimer.Stop() {
			cclog.ComponentDebug(s.name, "unexpected: the flushTimer was already running?")
		}

		// Run a batched flush for all lines that have arrived in the last flush delay interval
		s.flushTimer = time.AfterFunc(s.flushDelay, func() {
			if err := s.Flush(); err != nil {
				cclog.ComponentError(s.name, "flush failed:", err.Error())
			}
		})
	}

	// Append metric to batch slice
	p := m.ToPoint(s.meta_as_tags)
	s.lock.Lock()
	s.batch = append(s.batch, p)
	s.lock.Unlock()

	// Flush synchronously if "flush_delay" is zero
	if s.flushDelay == 0 {
		return s.Flush()
	}

	// Flush if batch size is reached
	if len(s.batch) == s.config.BatchSize {
		return s.Flush()
	}

	return nil
}

// Flush sends all metrics buffered in batch slice to InfluxDB server
func (s *InfluxSink) Flush() error {

	// Lock access to batch slice
	s.lock.Lock()
	defer s.lock.Unlock()

	// Nothing to do, batch slice is empty
	if len(s.batch) == 0 {
		return nil
	}

	// Send metrics from batch slice
	err := s.writeApi.WritePoint(context.Background(), s.batch...)
	if err != nil {
		cclog.ComponentError(s.name, "flush failed:", err.Error())
		return err
	}

	// Clear batch slice
	for i := range s.batch {
		s.batch[i] = nil
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

// NewInfluxSink create a new InfluxDB sink
func NewInfluxSink(name string, config json.RawMessage) (Sink, error) {
	s := new(InfluxSink)
	s.name = fmt.Sprintf("InfluxSink(%s)", name)

	// Set config default values
	s.config.BatchSize = 100
	s.config.FlushDelay = "1s"

	// Read config
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

	if len(s.config.Host) == 0 {
		return nil, errors.New("Missing host configuration required by InfluxSink")
	}
	if len(s.config.Port) == 0 {
		return nil, errors.New("Missing port configuration required by InfluxSink")
	}
	if len(s.config.Database) == 0 {
		return nil, errors.New("Missing database configuration required by InfluxSink")
	}
	if len(s.config.Organization) == 0 {
		return nil, errors.New("Missing organization configuration required by InfluxSink")
	}
	if len(s.config.Password) == 0 {
		return nil, errors.New("Missing password configuration required by InfluxSink")
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

	// Configure flush delay duration
	if len(s.config.FlushDelay) > 0 {
		t, err := time.ParseDuration(s.config.FlushDelay)
		if err == nil {
			s.flushDelay = t
		}
	}

	// allocate batch slice
	s.batch = make([]*write.Point, 0, s.config.BatchSize)

	// Connect to InfluxDB server
	if err := s.connect(); err != nil {
		return nil, fmt.Errorf("unable to connect: %v", err)
	}
	return s, nil
}
