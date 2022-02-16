//go:build !ganglia

package sinks

import (
	"encoding/json"
	"errors"

	//	"time"

	lp "github.com/ClusterCockpit/cc-metric-collector/internal/ccMetric"
)

type GangliaSink struct {
	sink
}

func (s *GangliaSink) Init(config json.RawMessage) error {
	return errors.New("sink 'ganglia' not implemented, rebuild with tag 'ganglia'")
}

func (s *GangliaSink) Write(point lp.CCMetric) error {
	return errors.New("sink 'ganglia' not implemented, rebuild with tag 'ganglia'")
}

func (s *GangliaSink) Flush() error {
	return errors.New("sink 'ganglia' not implemented, rebuild with tag 'ganglia'")
}

func (s *GangliaSink) Close() {
}
