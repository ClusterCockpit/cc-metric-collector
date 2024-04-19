package sinks

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	cclog "github.com/ClusterCockpit/cc-metric-collector/pkg/ccLogger"
	lp "github.com/ClusterCockpit/cc-metric-collector/pkg/ccMetric"
	"github.com/go-mqtt/mqtt"
	influx "github.com/influxdata/line-protocol/v2/lineprotocol"
	"golang.org/x/exp/slices"
)

type MqttSinkConfig struct {
	// defines JSON tags for 'type' and 'meta_as_tags' (string list)
	// See: metricSink.go
	defaultSinkConfig
	// Additional config options, for MqttSink
	ClientID             string `json:"client_id"`
	PersistenceDirectory string `json:"persistence_directory,omitempty"`
	// Maximum number of points sent to server in single request.
	// Default: 1000
	BatchSize int `json:"batch_size,omitempty"`

	// Time interval for delayed sending of metrics.
	// If the buffers are already filled before the end of this interval,
	// the metrics are sent without further delay.
	// Default: 1s
	FlushInterval string `json:"flush_delay,omitempty"`
	flushDelay    time.Duration

	DialProtocol string `json:"dial_protocol"`
	Hostname     string `json:"hostname"`
	Port         int    `json:"port"`
	PauseTimeout string `json:"pause_timeout"`
	pauseTimeout time.Duration
	KeepAlive    uint16 `json:"keep_alive_seconds"`
	Username     string `json:"username,omitempty"`
	Password     string `json:"password,omitempty"`
}

type MqttSink struct {
	// declares elements 	'name' and 'meta_as_tags' (string to bool map!)
	sink
	config MqttSinkConfig // entry point to the MqttSinkConfig
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

	client     *mqtt.Client
	mqttconfig mqtt.Config
}

// Implement functions required for Sink interface
// Write(...), Flush(), Close()
// See: metricSink.go

// Code to submit a single CCMetric to the sink
func (s *MqttSink) Write(m lp.CCMetric) error {

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

	// Check for encoder errors
	if err := s.encoder.Err(); err != nil {
		// Unlock encoder usage
		s.encoderLock.Unlock()

		return fmt.Errorf("encoding failed: %v", err)
	}
	s.numRecordsInEncoder++

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

// If the sink uses batched sends internally, you can tell to flush its buffers
func (s *MqttSink) Flush() error {

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
		//startTime := time.Now()
		for {
			exchange, err := s.client.PublishAtLeastOnce(buf, s.config.ClientID)
			switch {
			case err == nil:
				return

			case mqtt.IsDeny(err), errors.Is(err, mqtt.ErrClosed):
				return

			case errors.Is(err, mqtt.ErrMax):
				time.Sleep(s.config.pauseTimeout)

			default:
				time.Sleep(s.config.pauseTimeout)
				continue
			}

			for err := range exchange {
				if errors.Is(err, mqtt.ErrClosed) {
					return
				}

			}
			return
		}
	}()
	return nil
}

// Close sink: close network connection, close files, close libraries, ...
func (s *MqttSink) Close() {

	cclog.ComponentDebug(s.name, "CLOSE")

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
	s.client = nil
}

// New function to create a new instance of the sink
// Initialize the sink by giving it a name and reading in the config JSON
func NewMqttSink(name string, config json.RawMessage) (Sink, error) {
	s := new(MqttSink)

	// Set name of sampleSink
	// The name should be chosen in such a way that different instances of MqttSink can be distinguished
	s.name = fmt.Sprintf("MqttSink(%s)", name) // Always specify a name here

	// Set defaults in s.config
	// Allow overwriting these defaults by reading config JSON

	s.config.PauseTimeout = "4s"
	s.config.pauseTimeout = time.Duration(4) * time.Second
	s.config.DialProtocol = "tcp"
	s.config.Hostname = "localhost"
	s.config.Port = 1883

	// Read in the config JSON
	if len(config) > 0 {
		d := json.NewDecoder(bytes.NewReader(config))
		d.DisallowUnknownFields()
		if err := d.Decode(&s.config); err != nil {
			cclog.ComponentError(s.name, "Error reading config:", err.Error())
			return nil, err
		}
	}

	// Create lookup map to use meta infos as tags in the output metric
	s.meta_as_tags = make(map[string]bool)
	for _, k := range s.config.MetaAsTags {
		s.meta_as_tags[k] = true
	}

	// Check if all required fields in the config are set
	// E.g. use 'len(s.config.Option) > 0' for string settings
	if t, err := time.ParseDuration(s.config.PauseTimeout); err == nil {
		s.config.pauseTimeout = t
	} else {
		err := fmt.Errorf("to parse duration for PauseTimeout: %s", s.config.PauseTimeout)
		cclog.ComponentError(s.name, err.Error())
		return nil, err
	}
	if t, err := time.ParseDuration(s.config.FlushInterval); err == nil {
		s.config.flushDelay = t
	} else {
		err := fmt.Errorf("to parse duration for FlushInterval: %s", s.config.FlushInterval)
		cclog.ComponentError(s.name, err.Error())
		return nil, err
	}

	switch s.config.DialProtocol {
	case "tcp", "udp":
	default:
		err := errors.New("failed to parse dial protocol, allowed: tcp, udp")
		cclog.ComponentError(s.name, err.Error())
		return nil, err
	}

	var persistence mqtt.Persistence
	if len(s.config.PersistenceDirectory) > 0 {
		persistence = mqtt.FileSystem(s.config.PersistenceDirectory)
	} else {
		tmpdir, err := os.MkdirTemp("", "mqtt")
		if err == nil {
			persistence = mqtt.FileSystem(tmpdir)
		}
	}

	// Establish connection to the server, library, ...
	// Check required files exist and lookup path(s) of executable(s)

	dialer := mqtt.NewDialer(s.config.DialProtocol, net.JoinHostPort(s.config.Hostname, fmt.Sprintf("%d", s.config.Port)))

	s.mqttconfig = mqtt.Config{
		Dialer:       dialer,
		PauseTimeout: s.config.pauseTimeout,
		KeepAlive:    uint16(s.config.KeepAlive),
	}
	if len(s.config.Username) > 0 {
		s.mqttconfig.UserName = s.config.Username
	}
	if len(s.config.Password) > 0 {
		s.mqttconfig.Password = []byte(s.config.Password)
	}

	client, err := mqtt.InitSession(s.config.ClientID, persistence, &s.mqttconfig)
	if err != nil {
		return nil, err
	}
	s.client = client

	// Return (nil, meaningful error message) in case of errors
	return s, nil
}
