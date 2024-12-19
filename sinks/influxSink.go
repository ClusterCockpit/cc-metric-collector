package sinks

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	lp "github.com/ClusterCockpit/cc-energy-manager/pkg/cc-message"
	cclog "github.com/ClusterCockpit/cc-metric-collector/pkg/ccLogger"
	mp "github.com/ClusterCockpit/cc-metric-collector/pkg/messageProcessor"
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
		flushDelay    time.Duration

		// Influx client options:

		// HTTP request timeout
		HTTPRequestTimeout string `json:"http_request_timeout"`
		// Retry interval
		InfluxRetryInterval string `json:"retry_interval,omitempty"`
		// maximum delay between each retry attempt
		InfluxMaxRetryInterval string `json:"max_retry_interval,omitempty"`
		// base for the exponential retry delay
		InfluxExponentialBase uint `json:"retry_exponential_base,omitempty"`
		// maximum count of retry attempts of failed writes
		InfluxMaxRetries uint `json:"max_retries,omitempty"`
		// maximum total retry timeout
		InfluxMaxRetryTime string `json:"max_retry_time,omitempty"`
		// Specify whether to use GZip compression in write requests
		InfluxUseGzip bool `json:"use_gzip"`
		// Timestamp precision
		Precision string `json:"precision,omitempty"`
	}

	// influx line protocol encoder
	encoder influx.Encoder
	// number of records stored in the encoder
	numRecordsInEncoder int
	// List of tags and meta data tags which should be used as tags
	extended_tag_list []key_value_pair
	// Flush() runs in another goroutine and accesses the influx line protocol encoder,
	// so this encoderLock has to protect the encoder and numRecordsInEncoder
	encoderLock sync.Mutex

	// timer to run Flush()
	flushTimer *time.Timer
	// Lock to assure that only one timer is running at a time
	timerLock sync.Mutex

	// WaitGroup to ensure only one send operation is running at a time
	sendWaitGroup sync.WaitGroup
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

	// set HTTP request timeout
	if len(s.config.HTTPRequestTimeout) > 0 {
		if t, err := time.ParseDuration(s.config.HTTPRequestTimeout); err == nil {
			httpRequestTimeout := uint(t.Seconds())
			clientOptions.SetHTTPRequestTimeout(httpRequestTimeout)
		} else {
			cclog.ComponentError(s.name, "connect():", "Failed to parse duration for HTTP RequestTimeout: ", s.config.HTTPRequestTimeout)
		}
	}
	cclog.ComponentDebug(
		s.name,
		"connect():",
		"Influx client options HTTPRequestTimeout:",
		time.Second*time.Duration(clientOptions.HTTPRequestTimeout()))

	// Set retry interval
	if len(s.config.InfluxRetryInterval) > 0 {
		if t, err := time.ParseDuration(s.config.InfluxRetryInterval); err == nil {
			influxRetryInterval := uint(t.Milliseconds())
			clientOptions.SetRetryInterval(influxRetryInterval)
		} else {
			cclog.ComponentError(s.name, "connect():", "Failed to parse duration for Influx RetryInterval: ", s.config.InfluxRetryInterval)
		}
	}
	cclog.ComponentDebug(
		s.name,
		"connect():",
		"Influx client options RetryInterval:",
		time.Millisecond*time.Duration(clientOptions.RetryInterval()))

	// Set the maximum delay between each retry attempt
	if len(s.config.InfluxMaxRetryInterval) > 0 {
		if t, err := time.ParseDuration(s.config.InfluxMaxRetryInterval); err == nil {
			influxMaxRetryInterval := uint(t.Milliseconds())
			clientOptions.SetMaxRetryInterval(influxMaxRetryInterval)
		} else {
			cclog.ComponentError(s.name, "connect():", "Failed to parse duration for Influx MaxRetryInterval: ", s.config.InfluxMaxRetryInterval)
		}
	}
	cclog.ComponentDebug(
		s.name,
		"connect():",
		"Influx client options MaxRetryInterval:",
		time.Millisecond*time.Duration(clientOptions.MaxRetryInterval()))

	// Set the base for the exponential retry delay
	if s.config.InfluxExponentialBase != 0 {
		clientOptions.SetExponentialBase(s.config.InfluxExponentialBase)
	}
	cclog.ComponentDebug(
		s.name,
		"connect():",
		"Influx client options ExponentialBase:",
		clientOptions.ExponentialBase())

	// Set maximum count of retry attempts of failed writes
	if s.config.InfluxMaxRetries != 0 {
		clientOptions.SetMaxRetries(s.config.InfluxMaxRetries)
	}
	cclog.ComponentDebug(
		s.name,
		"connect():",
		"Influx client options MaxRetries:",
		clientOptions.MaxRetries())

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
	cclog.ComponentDebug(
		s.name,
		"connect():",
		"Influx client options MaxRetryTime:",
		time.Millisecond*time.Duration(clientOptions.MaxRetryTime()))

	// Specify whether to use GZip compression in write requests
	clientOptions.SetUseGZip(s.config.InfluxUseGzip)
	cclog.ComponentDebug(
		s.name,
		"connect():",
		"Influx client options UseGZip:",
		clientOptions.UseGZip())

	// Do not check InfluxDB certificate
	clientOptions.SetTLSConfig(
		&tls.Config{
			InsecureSkipVerify: true,
		},
	)

	// Set time precision
	precision := time.Second
	if len(s.config.Precision) > 0 {
		switch s.config.Precision {
		case "s":
			precision = time.Second
		case "ms":
			precision = time.Millisecond
		case "us":
			precision = time.Microsecond
		case "ns":
			precision = time.Nanosecond
		}
	}
	clientOptions.SetPrecision(precision)

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

// Write sends metric m in influxDB line protocol
func (s *InfluxSink) Write(msg lp.CCMessage) error {

	m, err := s.mp.ProcessMessage(msg)
	if err == nil && m != nil {
		// Lock for encoder usage
		s.encoderLock.Lock()

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
		// for _, key := range s.config.MetaAsTags {
		// 	if value, ok := m.GetMeta(key); ok {
		// 		s.extended_tag_list =
		// 			append(
		// 				s.extended_tag_list,
		// 				key_value_pair{
		// 					key:   key,
		// 					value: value,
		// 				},
		// 			)
		// 	}
		// }

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

		// Check for encoder errors
		if err := s.encoder.Err(); err != nil {
			// Unlock encoder usage
			s.encoderLock.Unlock()

			return fmt.Errorf("encoding failed: %v", err)
		}
		s.numRecordsInEncoder++
	}

	if s.config.flushDelay == 0 {
		// Unlock encoder usage
		s.encoderLock.Unlock()

		// Directly flush if no flush delay is configured
		return s.Flush()
	} else if s.numRecordsInEncoder == s.config.BatchSize {
		// Unlock encoder usage
		s.encoderLock.Unlock()

		// Stop flush timer
		if s.flushTimer != nil {
			if ok := s.flushTimer.Stop(); ok {
				cclog.ComponentDebug(s.name, "Write(): Stopped flush timer. Batch size limit reached before flush delay")
				s.timerLock.Unlock()
			}
		}

		// Flush if batch size is reached
		return s.Flush()
	} else if s.timerLock.TryLock() {

		// Setup flush timer when flush delay is configured
		// and no other timer is already running
		if s.flushTimer != nil {

			// Restarting existing flush timer
			cclog.ComponentDebug(s.name, "Write(): Restarting flush timer")
			s.flushTimer.Reset(s.config.flushDelay)
		} else {

			// Creating and starting flush timer
			cclog.ComponentDebug(s.name, "Write(): Starting new flush timer")
			s.flushTimer = time.AfterFunc(
				s.config.flushDelay,
				func() {
					defer s.timerLock.Unlock()
					cclog.ComponentDebug(s.name, "Starting flush triggered by flush timer")
					if err := s.Flush(); err != nil {
						cclog.ComponentError(s.name, "Flush triggered by flush timer: flush failed:", err)
					}
				})
		}
	}

	// Unlock encoder usage
	s.encoderLock.Unlock()
	return nil
}

// Flush sends all metrics stored in encoder to InfluxDB server
func (s *InfluxSink) Flush() error {

	// Lock for encoder usage
	// Own lock for as short as possible: the time it takes to clone the buffer.
	s.encoderLock.Lock()

	buf := slices.Clone(s.encoder.Bytes())
	numRecordsInBuf := s.numRecordsInEncoder
	s.encoder.Reset()
	s.numRecordsInEncoder = 0

	// Unlock encoder usage
	s.encoderLock.Unlock()

	if len(buf) == 0 {
		return nil
	}

	cclog.ComponentDebug(s.name, "Flush(): Flushing", numRecordsInBuf, "metrics")

	// Asynchron send of encoder metrics
	s.sendWaitGroup.Add(1)
	go func() {
		defer s.sendWaitGroup.Done()
		startTime := time.Now()
		err := s.writeApi.WriteRecord(context.Background(), string(buf))
		if err != nil {
			cclog.ComponentError(
				s.name,
				"Flush():",
				"Flush failed:", err,
				"(number of records =", numRecordsInBuf,
				", buffer size =", len(buf),
				", send duration =", time.Since(startTime),
				")",
			)
			return
		}
	}()

	return nil
}

func (s *InfluxSink) Close() {
	cclog.ComponentDebug(s.name, "Closing InfluxDB connection")

	// Stop existing timer and immediately flush
	if s.flushTimer != nil {
		if ok := s.flushTimer.Stop(); ok {
			s.timerLock.Unlock()
		}
	}

	// Flush
	if err := s.Flush(); err != nil {
		cclog.ComponentError(s.name, "Close():", "Flush failed:", err)
	}

	// Wait for send operations to finish
	s.sendWaitGroup.Wait()

	s.client.Close()
}

// NewInfluxSink create a new InfluxDB sink
func NewInfluxSink(name string, config json.RawMessage) (Sink, error) {
	s := new(InfluxSink)
	s.name = fmt.Sprintf("InfluxSink(%s)", name)

	// Set config default values
	s.config.BatchSize = 1000
	s.config.FlushInterval = "1s"
	s.config.Precision = "s"

	// Read config
	if len(config) > 0 {
		d := json.NewDecoder(bytes.NewReader(config))
		d.DisallowUnknownFields()
		if err := d.Decode(&s.config); err != nil {
			cclog.ComponentError(s.name, "Error reading config:", err.Error())
			return nil, err
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
	p, err := mp.NewMessageProcessor()
	if err != nil {
		return nil, fmt.Errorf("initialization of message processor failed: %v", err.Error())
	}
	s.mp = p

	if len(s.config.MessageProcessor) > 0 {
		err = p.FromConfigJSON(s.config.MessageProcessor)
		if err != nil {
			return nil, fmt.Errorf("failed parsing JSON for message processor: %v", err.Error())
		}
	}
	for _, k := range s.config.MetaAsTags {
		s.mp.AddMoveMetaToTags("true", k, k)
	}

	// Configure flush delay duration
	if len(s.config.FlushInterval) > 0 {
		t, err := time.ParseDuration(s.config.FlushInterval)
		if err == nil {
			s.config.flushDelay = t
		}
	}

	if !(s.config.BatchSize > 0) {
		return s, fmt.Errorf("batch_size=%d in InfluxDB config must be > 0", s.config.BatchSize)
	}

	// Connect to InfluxDB server
	if err := s.connect(); err != nil {
		return s, fmt.Errorf("unable to connect: %v", err)
	}

	// Configure influx line protocol encoder
	s.encoder.SetPrecision(influx.Nanosecond)
	s.extended_tag_list = make([]key_value_pair, 0)

	return s, nil
}
