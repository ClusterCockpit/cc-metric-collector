package receivers

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	cclog "github.com/ClusterCockpit/cc-metric-collector/pkg/ccLogger"
	lp "github.com/ClusterCockpit/cc-metric-collector/pkg/ccMetric"
	influx "github.com/influxdata/line-protocol"
	nats "github.com/nats-io/nats.go"
)

type NatsReceiverConfig struct {
	Type    string `json:"type"`
	Addr    string `json:"address"`
	Port    string `json:"port"`
	Subject string `json:"subject"`
}

type NatsReceiver struct {
	receiver
	nc      *nats.Conn
	handler *influx.MetricHandler
	parser  *influx.Parser
	meta    map[string]string
	config  NatsReceiverConfig
}

var DefaultTime = func() time.Time {
	return time.Unix(42, 0)
}

// Start subscribes to the configured NATS subject
// Messages wil be handled by r._NatsReceive
func (r *NatsReceiver) Start() {
	cclog.ComponentDebug(r.name, "START")
	r.nc.Subscribe(r.config.Subject, r._NatsReceive)
}

// _NatsReceive receives subscribed messages from the NATS server
func (r *NatsReceiver) _NatsReceive(m *nats.Msg) {
	metrics, err := r.parser.Parse(m.Data)
	if err == nil {
		for _, m := range metrics {
			y := lp.FromInfluxMetric(m)
			for k, v := range r.meta {
				y.AddMeta(k, v)
			}
			if r.sink != nil {
				r.sink <- y
			}
		}
	}
}

// Close closes the connection to the NATS server
func (r *NatsReceiver) Close() {
	if r.nc != nil {
		cclog.ComponentDebug(r.name, "CLOSE")
		r.nc.Close()
	}
}

// NewNatsReceiver creates a new Receiver which subscribes to messages from a NATS server
func NewNatsReceiver(name string, config json.RawMessage) (Receiver, error) {
	r := new(NatsReceiver)
	r.name = fmt.Sprintf("NatsReceiver(%s)", name)

	// Read configuration file, allow overwriting default config
	r.config.Addr = "localhost"
	r.config.Port = "4222"
	if len(config) > 0 {
		err := json.Unmarshal(config, &r.config)
		if err != nil {
			cclog.ComponentError(r.name, "Error reading config:", err.Error())
			return nil, err
		}
	}
	if len(r.config.Addr) == 0 ||
		len(r.config.Port) == 0 ||
		len(r.config.Subject) == 0 {
		return nil, errors.New("not all configuration variables set required by NatsReceiver")
	}

	// Set metadata
	r.meta = map[string]string{
		"source": r.name,
	}

	// Connect to NATS server
	url := fmt.Sprintf("nats://%s:%s", r.config.Addr, r.config.Port)
	cclog.ComponentDebug(r.name, "NewNatsReceiver", url, "Subject", r.config.Subject)
	if nc, err := nats.Connect(url); err == nil {
		r.nc = nc
	} else {
		r.nc = nil
		return nil, err
	}

	r.handler = influx.NewMetricHandler()
	r.parser = influx.NewParser(r.handler)
	r.parser.SetTimeFunc(DefaultTime)
	return r, nil
}
