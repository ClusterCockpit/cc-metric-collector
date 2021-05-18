package receivers

import (
	//	"time"
	s "github.com/ClusterCockpit/cc-metric-collector/sinks"
	influx "github.com/influxdata/line-protocol"
)

type ReceiverConfig struct {
	Addr     string `json:"address"`
	Port     string `json:"port"`
	Database string `json:"database"`
	Type     string `json:"type"`
}

type Receiver struct {
	name         string
	addr         string
	port         string
	database     string
	organization string
	sink         s.SinkFuncs
}

type ReceiverFuncs interface {
	Init(config ReceiverConfig, sink s.SinkFuncs) error
	Start()
	Close()
}

func Tags2Map(metric influx.Metric) map[string]string {
	tags := make(map[string]string)
	for _, t := range metric.TagList() {
		tags[t.Key] = t.Value
	}
	return tags
}

func Fields2Map(metric influx.Metric) map[string]interface{} {
	fields := make(map[string]interface{})
	for _, f := range metric.FieldList() {
		fields[f.Key] = f.Value
	}
	return fields
}
