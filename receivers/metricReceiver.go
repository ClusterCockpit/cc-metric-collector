package receivers

import (
	lp "github.com/ClusterCockpit/cc-energy-manager/pkg/cc-message"
)

type defaultReceiverConfig struct {
	Type string `json:"type"`
}

// Receiver configuration: Listen address, port
type ReceiverConfig struct {
	Addr         string `json:"address"`
	Port         string `json:"port"`
	Database     string `json:"database"`
	Organization string `json:"organization,omitempty"`
	Type         string `json:"type"`
}

type receiver struct {
	name string
	sink chan lp.CCMessage
}

type Receiver interface {
	Start()
	Close()                        // Close / finish metric receiver
	Name() string                  // Name of the metric receiver
	SetSink(sink chan lp.CCMessage) // Set sink channel
}

// Name returns the name of the metric receiver
func (r *receiver) Name() string {
	return r.name
}

// SetSink set the sink channel
func (r *receiver) SetSink(sink chan lp.CCMessage) {
	r.sink = sink
}
