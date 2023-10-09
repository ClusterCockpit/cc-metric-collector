package sinks

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	cclog "github.com/ClusterCockpit/cc-metric-collector/pkg/ccLogger"
	lp "github.com/ClusterCockpit/cc-metric-collector/pkg/ccMetric"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	influxdb2Api "github.com/influxdata/influxdb-client-go/v2/api"
	influx "github.com/influxdata/line-protocol/v2/lineprotocol"
	"golang.org/x/exp/slices"
)

type InfluxSink struct {
	sink
	client   influxdb2.Client
	writeApi influxdb2Api.WriteAPIBlocking
	config   struct {
		defaultSinkConfig
		Host         string `json:"host,omitempty"`
		Port         string `json:"port,omitempty"`
		Database     string `json:"database,omitempty"`
		User         string `json:"user,omitempty"`
		Password     string `json:"password,omitempty"`
		Organization string `json:"organization,omitempty"`
		SSL          bool   `json:"ssl,omitempty"`
		// Maximum number of points sent to server in single request.
		// Default: 1000
		BatchSize int `json:"batch_size,omitempty"`
		// Time interval for delayed sending of metrics.
		// If the buffers are already filled before the end of this interval,
		// the metrics are sent without further delay.
		// Default: 1s
		FlushInterval string `json:"flush_delay,omitempty"`
		// Number of metrics that are dropped when buffer is full
		// Default: 100
		DropRate int `json:"drop_rate,omitempty"`

		// Influx client options:

		// maximum delay between each retry attempt
		InfluxMaxRetryInterval string `json:"retry_interval,omitempty"`
		// base for the exponential retry delay
		InfluxExponentialBase uint `json:"retry_exponential_base,omitempty"`
		// maximum count of retry attempts of failed writes
		InfluxMaxRetries uint `json:"max_retries,omitempty"`
		// maximum total retry timeout
		InfluxMaxRetryTime string `json:"max_retry_time,omitempty"`
	}
	batch           []string
	flushTimer      *time.Timer
	flushDelay      time.Duration
	batchMutex      sync.Mutex // Flush() runs in another goroutine, so this lock has to protect the buffer
	flushTimerMutex sync.Mutex // Ensure only one flush timer is running
	// influx line protocol encoder
	encoder influx.Encoder
	// List of tags and meta data tags which should be used as tags
	extended_tag_list []key_value_pair
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
	cclog.ComponentDebug(s.name, "connect():",
		"Using URI='"+uri+"'",
		"Org='"+s.config.Organization+"'",
		"Bucket='"+s.config.Database+"'")

	// Set influxDB client options
	clientOptions := influxdb2.DefaultOptions()

	// Set the maximum delay between each retry attempt
	if len(s.config.InfluxMaxRetryInterval) > 0 {
		if t, err := time.ParseDuration(s.config.InfluxMaxRetryInterval); err == nil {
			influxMaxRetryInterval := uint(t.Milliseconds())
			cclog.ComponentDebug(s.name, "connect():", "Influx MaxRetryInterval", s.config.InfluxMaxRetryInterval)
			clientOptions.SetMaxRetryInterval(influxMaxRetryInterval)
		} else {
			cclog.ComponentError(s.name, "connect():", "Failed to parse duration for Influx MaxRetryInterval: ", s.config.InfluxMaxRetryInterval)
		}
	}

	// Set the base for the exponential retry delay
	if s.config.InfluxExponentialBase != 0 {
		cclog.ComponentDebug(s.name, "connect():", "Influx Exponential Base", s.config.InfluxExponentialBase)
		clientOptions.SetExponentialBase(s.config.InfluxExponentialBase)
	}

	// Set maximum count of retry attempts of failed writes
	if s.config.InfluxMaxRetries != 0 {
		cclog.ComponentDebug(s.name, "connect():", "Influx Max Retries", s.config.InfluxMaxRetries)
		clientOptions.SetMaxRetries(s.config.InfluxMaxRetries)
	}

	// Set the maximum total retry timeout
	if len(s.config.InfluxMaxRetryTime) > 0 {
		if t, err := time.ParseDuration(s.config.InfluxMaxRetryTime); err == nil {
			influxMaxRetryTime := uint(t.Milliseconds())
			cclog.ComponentDebug(s.name, "connect():", "MaxRetryTime", s.config.InfluxMaxRetryTime)
			clientOptions.SetMaxRetryTime(influxMaxRetryTime)
		} else {
			cclog.ComponentError(s.name, "connect():", "Failed to parse duration for Influx MaxRetryInterval: ", s.config.InfluxMaxRetryInterval)
		}
	}

	// Do not check InfluxDB certificate
	clientOptions.SetTLSConfig(
		&tls.Config{
			InsecureSkipVerify: true,
		},
	)

	// Set time precision
	clientOptions.SetPrecision(time.Nanosecond)

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
	if s.flushDelay != 0 && s.flushTimerMutex.TryLock() {

		// Setup flush timer when flush delay is configured
		// and no other timer is already running
		if s.flushTimer != nil {

			// Restarting existing flush timer
			cclog.ComponentDebug(s.name, "Write():", "Restarting flush timer")
			s.flushTimer.Reset(s.flushDelay)
		} else {

			// Creating and starting flush timer
			cclog.ComponentDebug(s.name, "Write():", "Starting new flush timer")
			s.flushTimer = time.AfterFunc(
				s.flushDelay,
				func() {
					defer s.flushTimerMutex.Unlock()
					cclog.ComponentDebug(s.name, "Starting flush in flush timer")
					if err := s.Flush(); err != nil {
						cclog.ComponentError(s.name, "Flush timer: flush failed:", err)
					}
				})
		}
	}

	// Protect access to batch slice
	s.batchMutex.Lock()

	// batch slice full, dropping oldest metric(s)
	// e.g. when previous flushes failed and batch slice was not cleared
	if len(s.batch) == s.config.BatchSize {
		newSize := s.config.BatchSize - s.config.DropRate

		for i := 0; i < newSize; i++ {
			s.batch[i] = s.batch[i+s.config.DropRate]
		}
		for i := newSize; i < s.config.BatchSize; i++ {
			s.batch[i] = ""
		}
		s.batch = s.batch[:newSize]
		cclog.ComponentError(s.name, "Write():", "Batch slice full, dropping", s.config.DropRate, "oldest metric(s)")
	}

	// Encode measurement name
	s.encoder.StartLine(m.Name())

	// copy tags and meta data which should be used as tags
	s.extended_tag_list = s.extended_tag_list[:0]
	for key, value := range m.Tags() {
		s.extended_tag_list =
			append(
				s.extended_tag_list,
				key_value_pair{
					key:   key,
					value: value,
				},
			)
	}
	for _, key := range s.config.MetaAsTags {
		if value, ok := m.GetMeta(key); ok {
			s.extended_tag_list =
				append(
					s.extended_tag_list,
					key_value_pair{
						key:   key,
						value: value,
					},
				)
		}
	}

	// Encode tags (they musts be in lexical order)
	slices.SortFunc(
		s.extended_tag_list,
		func(a key_value_pair, b key_value_pair) int {
			if a.key < b.key {
				return -1
			}
			if a.key > b.key {
				return +1
			}
			return 0
		},
	)
	for i := range s.extended_tag_list {
		s.encoder.AddTag(
			s.extended_tag_list[i].key,
			s.extended_tag_list[i].value,
		)
	}

	// Encode fields
	for key, value := range m.Fields() {
		s.encoder.AddField(key, influx.MustNewValue(value))
	}

	// Encode time stamp
	s.encoder.EndLine(m.Time())

	// Check that encoding worked
	if err := s.encoder.Err(); err != nil {
		cclog.ComponentError(s.name, "Write():", "Encoding failed:", err)

		// Unlock access to batch slice
		s.batchMutex.Unlock()
		return err
	}

	// Append metric to batch slice
	s.batch = append(s.batch,
		string(
			slices.Clone(
				s.encoder.Bytes())))
	s.encoder.Reset()

	// Flush synchronously if "flush_delay" is zero
	// or
	// Flush if batch size is reached
	if s.flushDelay == 0 ||
		len(s.batch) == s.config.BatchSize {

		// Stop flush timer
		if s.flushTimer != nil {
			if ok := s.flushTimer.Stop(); ok {
				s.flushTimerMutex.Unlock()
			}
		}

		// Unlock access to batch slice
		s.batchMutex.Unlock()
		return s.Flush()
	}

	// Unlock access to batch slice
	s.batchMutex.Unlock()
	return nil
}

// Flush sends all metrics buffered in batch slice to InfluxDB server
func (s *InfluxSink) Flush() error {
	cclog.ComponentDebug(s.name, "Flushing")

	// Lock access to batch slice
	s.batchMutex.Lock()
	defer s.batchMutex.Unlock()

	// Nothing to do, batch slice is empty
	if len(s.batch) == 0 {
		return nil
	}

	// Send metrics from batch slice
	err := s.writeApi.WriteRecord(context.Background(), strings.Join(s.batch, ""))
	if err != nil {
		cclog.ComponentError(s.name, "Flush():", "Flush of", len(s.batch), "metrics failed:", err)
		return err
	}

	// Clear batch slice
	for i := range s.batch {
		s.batch[i] = ""
	}
	s.batch = s.batch[:0]

	return nil
}

func (s *InfluxSink) Close() {
	cclog.ComponentDebug(s.name, "Closing InfluxDB connection")

	// Stop existing timer and immediately flush
	if s.flushTimer != nil {
		if ok := s.flushTimer.Stop(); ok {
			s.flushTimerMutex.Unlock()
		}
	}
	s.Flush()
	if err := s.Flush(); err != nil {
		cclog.ComponentError(s.name, "Close():", "Flush failed:", err)
	}

	s.client.Close()
}

// NewInfluxSink create a new InfluxDB sink
func NewInfluxSink(name string, config json.RawMessage) (Sink, error) {
	s := new(InfluxSink)
	s.name = fmt.Sprintf("InfluxSink(%s)", name)

	// Set config default values
	s.config.BatchSize = 1000
	s.config.FlushInterval = "1s"
	s.config.DropRate = 100

	// Read config
	if len(config) > 0 {
		err := json.Unmarshal(config, &s.config)
		if err != nil {
			return s, err
		}
	}

	if len(s.config.Host) == 0 {
		return s, errors.New("missing host configuration required by InfluxSink")
	}
	if len(s.config.Port) == 0 {
		return s, errors.New("missing port configuration required by InfluxSink")
	}
	if len(s.config.Database) == 0 {
		return s, errors.New("missing database configuration required by InfluxSink")
	}
	if len(s.config.Organization) == 0 {
		return s, errors.New("missing organization configuration required by InfluxSink")
	}
	if len(s.config.Password) == 0 {
		return s, errors.New("missing password configuration required by InfluxSink")
	}

	// Create lookup map to use meta infos as tags in the output metric
	s.meta_as_tags = make(map[string]bool)
	for _, k := range s.config.MetaAsTags {
		s.meta_as_tags[k] = true
	}

	// Configure flush delay duration
	if len(s.config.FlushInterval) > 0 {
		t, err := time.ParseDuration(s.config.FlushInterval)
		if err == nil {
			s.flushDelay = t
		}
	}

	if !(s.config.BatchSize > 0) {
		return s, fmt.Errorf("batch_size=%d in InfluxDB config must be > 0", s.config.BatchSize)
	}
	if !(s.config.DropRate > 0) {
		return s, fmt.Errorf("drop_rate=%d in InfluxDB config must be > 0", s.config.DropRate)
	}
	if !(s.config.BatchSize > s.config.DropRate) {
		return s, fmt.Errorf(
			"batch_size=%d must be greater then drop_rate=%d in InfluxDB config",
			s.config.BatchSize, s.config.DropRate)
	}

	// allocate batch slice
	s.batch = make([]string, 0, s.config.BatchSize)

	// Connect to InfluxDB server
	if err := s.connect(); err != nil {
		return s, fmt.Errorf("unable to connect: %v", err)
	}

	// Configure influx line protocol encoder
	s.encoder.SetPrecision(influx.Nanosecond)
	s.extended_tag_list = make([]key_value_pair, 0)

	return s, nil
}
