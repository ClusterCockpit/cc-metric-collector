//go:build !ganglia
// +build !ganglia

package sinks

import (
	"encoding/json"
	"errors"

	lp "github.com/ClusterCockpit/cc-metric-collector/internal/ccMetric"
)

type LibgangliaSink struct {
	sink
}

func (s *LibgangliaSink) Init(config json.RawMessage) error {
	return errors.New("sink 'libganglia' not implemented, rebuild with tag 'ganglia'")
}

func (s *LibgangliaSink) Write(point lp.CCMetric) error {
	return errors.New("sink 'libganglia' not implemented, rebuild with tag 'ganglia'")
}

func (s *LibgangliaSink) Flush() error {
	return errors.New("sink 'ganglia' not implemented, rebuild with tag 'ganglia'")
}

func (s *LibgangliaSink) Close() {
}
