package receivers

import (
	lp "github.com/ClusterCockpit/cc-metric-collector/internal/ccMetric"
)

type defaultReceiverConfig struct {
	Type string `json:"type"`
}

type ReceiverConfig struct {
	Addr         string `json:"address"`
	Port         string `json:"port"`
	Database     string `json:"database"`
	Organization string `json:"organization,omitempty"`
	Type         string `json:"type"`
}

type receiver struct {
	name string
	sink chan lp.CCMetric
}

type Receiver interface {
	Start()
	Close()
	Name() string
	SetSink(sink chan lp.CCMetric)
}

func (r *receiver) Name() string {
	return r.name
}

func (r *receiver) SetSink(sink chan lp.CCMetric) {
	r.sink = sink
}
