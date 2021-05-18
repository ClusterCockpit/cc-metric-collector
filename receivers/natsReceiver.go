package receivers

import (
	s "github.com/ClusterCockpit/cc-metric-collector/sinks"
	protocol "github.com/influxdata/line-protocol"
	nats "github.com/nats-io/nats.go"
	"log"
	"time"
	"errors"
)

type NatsReceiver struct {
	Receiver
	nc *nats.Conn
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
	return err
}

func (r *NatsReceiver) Start() {
	log.Print("Start NATS Receiver")
	r.nc.Subscribe(r.database, r._NatsReceive)
}

func (r *NatsReceiver) _NatsReceive(m *nats.Msg) {
	handler := protocol.NewMetricHandler()
	parser := protocol.NewParser(handler)
	parser.SetTimeFunc(DefaultTime)
	metrics, err := parser.Parse(m.Data)
	if err == nil {
		for _, m := range metrics {
			tags := Tags2Map(m)
			fields := Fields2Map(m)
			r.sink.Write(m.Name(), tags, fields, m.Time())
		}
	}
}

func (r *NatsReceiver) Close() {
	if r.nc != nil {
		log.Print("Close NATS Receiver")
		r.nc.Close()
	}
}
