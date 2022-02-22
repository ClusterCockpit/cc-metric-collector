package receivers

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	cclog "github.com/ClusterCockpit/cc-metric-collector/internal/ccLogger"
	lp "github.com/ClusterCockpit/cc-metric-collector/internal/ccMetric"
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

func (r *NatsReceiver) Init(name string, config json.RawMessage) error {
	r.name = fmt.Sprintf("NatsReceiver(%s)", name)
	r.config.Addr = nats.DefaultURL
	r.config.Port = "4222"
	if len(config) > 0 {
		err := json.Unmarshal(config, &r.config)
		if err != nil {
			cclog.ComponentError(r.name, "Error reading config:", err.Error())
			return err
		}
	}
	if len(r.config.Addr) == 0 ||
		len(r.config.Port) == 0 ||
		len(r.config.Subject) == 0 {
		return errors.New("not all configuration variables set required by NatsReceiver")
	}
	r.meta = map[string]string{"source": r.name}
	uri := fmt.Sprintf("%s:%s", r.config.Addr, r.config.Port)
	cclog.ComponentDebug(r.name, "INIT", uri, "Subject", r.config.Subject)
	nc, err := nats.Connect(uri)
	if err == nil {
		r.nc = nc
	} else {
		r.nc = nil
		return err
	}
	r.handler = influx.NewMetricHandler()
	r.parser = influx.NewParser(r.handler)
	r.parser.SetTimeFunc(DefaultTime)
	return err
}

func (r *NatsReceiver) Start() {
	cclog.ComponentDebug(r.name, "START")
	r.nc.Subscribe(r.config.Subject, r._NatsReceive)
}

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

func (r *NatsReceiver) Close() {
	if r.nc != nil {
		cclog.ComponentDebug(r.name, "CLOSE")
		r.nc.Close()
	}
}

func NewNatsReceiver(name string, config json.RawMessage) (Receiver, error) {
	r := new(NatsReceiver)
	err := r.Init(name, config)
	return r, err
}
