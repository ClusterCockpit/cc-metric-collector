package receivers

import (
	"errors"
	influx "github.com/influxdata/line-protocol"
	lp "github.com/ClusterCockpit/cc-metric-collector/internal/ccMetric"
	nats "github.com/nats-io/nats.go"
	"log"
	"time"
	"fmt"
)

type NatsReceiverConfig struct {
    Addr     string `json:"address"`
	Port     string `json:"port"`
	Database string `json:"database"`
}

type NatsReceiver struct {
	receiver
	nc      *nats.Conn
	handler *influx.MetricHandler
	parser  *influx.Parser
	meta    map[string]string
	config  ReceiverConfig
}

var DefaultTime = func() time.Time {
	return time.Unix(42, 0)
}

func (r *NatsReceiver) Init(config ReceiverConfig) error {
	r.name = "NatsReceiver"
	r.config = config
	if len(r.config.Addr) == 0 ||
		len(r.config.Port) == 0 ||
		len(r.config.Database) == 0 {
		return errors.New("Not all configuration variables set required by NatsReceiver")
	}
	r.meta = map[string]string{"source" : r.name}
	r.addr = r.config.Addr
	if len(r.addr) == 0 {
		r.addr = nats.DefaultURL
	}
	r.port = r.config.Port
	if len(r.port) == 0 {
		r.port = "4222"
	}
	log.Print("[NatsReceiver] INIT")
	uri := fmt.Sprintf("%s:%s", r.addr, r.port)
	nc, err := nats.Connect(uri)
	if err == nil {
		r.database = r.config.Database
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
	log.Print("[NatsReceiver] START")
	r.nc.Subscribe(r.database, r._NatsReceive)
}

func (r *NatsReceiver) _NatsReceive(m *nats.Msg) {
	metrics, err := r.parser.Parse(m.Data)
	if err == nil {
		for _, m := range metrics {
		    y := lp.FromInfluxMetric(m)
		    for k, v := range r.meta {
		        y.AddMeta(k, v)
		    }
			//y, err := lp.New(m.Name(), Tags2Map(m), r.meta, Fields2Map(m), m.Time())
			if r.sink != nil {
				r.sink <- y
			}
		}
	}
}

func (r *NatsReceiver) Close() {
	if r.nc != nil {
		log.Print("[NatsReceiver] CLOSE")
		r.nc.Close()
	}
}

