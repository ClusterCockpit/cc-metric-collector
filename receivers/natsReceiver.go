package receivers

import (
	"errors"
	s "github.com/ClusterCockpit/cc-metric-collector/sinks"
	lp "github.com/influxdata/line-protocol"
	nats "github.com/nats-io/nats.go"
	"log"
	"time"
)

type NatsReceiver struct {
	Receiver
	nc      *nats.Conn
	handler *lp.MetricHandler
	parser  *lp.Parser
}

var DefaultTime = func() time.Time {
	return time.Unix(42, 0)
}

func (r *NatsReceiver) Init(config ReceiverConfig, sink s.SinkFuncs) error {
	if len(config.Addr) == 0 ||
		len(config.Port) == 0 ||
		len(config.Database) == 0 {
		return errors.New("Not all configuration variables set required by NatsReceiver")
	}
	r.addr = config.Addr
	if len(r.addr) == 0 {
		r.addr = nats.DefaultURL
	}
	r.port = config.Port
	if len(r.port) == 0 {
		r.port = "4222"
	}
	log.Print("Init NATS Receiver")
	nc, err := nats.Connect(r.addr)
	if err == nil {
		r.database = config.Database
		r.sink = sink
		r.nc = nc
	} else {
		log.Print(err)
		r.nc = nil
	}
	r.handler = lp.NewMetricHandler()
	r.parser = lp.NewParser(r.handler)
	r.parser.SetTimeFunc(DefaultTime)
	return err
}

func (r *NatsReceiver) Start() {
	log.Print("Start NATS Receiver")
	r.nc.Subscribe(r.database, r._NatsReceive)
}

func (r *NatsReceiver) _NatsReceive(m *nats.Msg) {
	metrics, err := r.parser.Parse(m.Data)
	if err == nil {
		for _, m := range metrics {
			y, err := lp.New(m.Name(), Tags2Map(m), Fields2Map(m), m.Time())
			if err == nil {
				r.sink.Write(y)
			}
		}
	}
}

func (r *NatsReceiver) Close() {
	if r.nc != nil {
		log.Print("Close NATS Receiver")
		r.nc.Close()
	}
}
