package receivers

import (
	//	"time"
	lp "github.com/ClusterCockpit/cc-metric-collector/internal/ccMetric"
	influx "github.com/influxdata/line-protocol"
)

type ReceiverConfig struct {
	Addr         string `json:"address"`
	Port         string `json:"port"`
	Database     string `json:"database"`
	Organization string `json:"organization,omitempty"`
	Type         string `json:"type"`
}

type receiver struct {
	name         string
	addr         string
	port         string
	database     string
	organization string
	sink         chan *lp.CCMetric
}

type Receiver interface {
	Init(config ReceiverConfig) error
	Start()
	Close()
	Name() string
	SetSink(sink chan *lp.CCMetric)
}

func (r *receiver) Name() string {
	return r.name
}

func (r *receiver) SetSink(sink chan *lp.CCMetric) {
	r.sink = sink
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
