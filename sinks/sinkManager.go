package sinks

import (
	"encoding/json"
	"os"
	"sync"

	cclog "github.com/ClusterCockpit/cc-metric-collector/internal/ccLogger"
	lp "github.com/ClusterCockpit/cc-metric-collector/internal/ccMetric"
)

var AvailableSinks = map[string]Sink{
	"influxdb": &InfluxSink{},
	"stdout":   &StdoutSink{},
	"nats":     &NatsSink{},
	"http":     &HttpSink{},
}

type sinkManager struct {
	input   chan lp.CCMetric
	outputs []Sink
	done    chan bool
	wg      *sync.WaitGroup
	config  []sinkConfig
}

type SinkManager interface {
	Init(wg *sync.WaitGroup, sinkConfigFile string) error
	AddInput(input chan lp.CCMetric)
	AddOutput(config json.RawMessage) error
	Start()
	Close()
}

func (sm *sinkManager) Init(wg *sync.WaitGroup, sinkConfigFile string) error {
	sm.input = nil
	sm.outputs = make([]Sink, 0)
	sm.done = make(chan bool)
	sm.wg = wg
	sm.config = make([]sinkConfig, 0)
	if len(sinkConfigFile) > 0 {
		configFile, err := os.Open(sinkConfigFile)
		if err != nil {
			cclog.ComponentError("SinkManager", err.Error())
			return err
		}
		defer configFile.Close()
		jsonParser := json.NewDecoder(configFile)
		var rawConfigs []json.RawMessage
		err = jsonParser.Decode(&rawConfigs)
		if err != nil {
			cclog.ComponentError("SinkManager", err.Error())
			return err
		}
		for _, raw := range rawConfigs {
			err = sm.AddOutput(raw)
			if err != nil {
				continue
			}
		}
	}
	return nil
}

func (sm *sinkManager) Start() {
	sm.wg.Add(1)
	batchcount := 20
	go func() {
		done := func() {
			for _, s := range sm.outputs {
				s.Close()
			}
			cclog.ComponentDebug("SinkManager", "DONE")
			sm.wg.Done()
		}
		for {
			select {
			case <-sm.done:
				done()
				return
			case p := <-sm.input:
				cclog.ComponentDebug("SinkManager", "WRITE", p)
				for _, s := range sm.outputs {
					s.Write(p)
				}
				if batchcount == 0 {
					cclog.ComponentDebug("SinkManager", "FLUSH")
					for _, s := range sm.outputs {
						s.Flush()
					}
					batchcount = 20
				}
				batchcount--
			}
		}
	}()
	cclog.ComponentDebug("SinkManager", "STARTED")
}

func (sm *sinkManager) AddInput(input chan lp.CCMetric) {
	sm.input = input
}

func (sm *sinkManager) AddOutput(rawConfig json.RawMessage) error {
	var err error
	var config sinkConfig
	if len(rawConfig) > 3 {
		err = json.Unmarshal(rawConfig, &config)
		if err != nil {
			cclog.ComponentError("SinkManager", "SKIP", config.Type, "JSON config error:", err.Error())
			return err
		}
	}
	if _, found := AvailableSinks[config.Type]; !found {
		cclog.ComponentError("SinkManager", "SKIP", config.Type, "unknown sink:", err.Error())
		return err
	}
	s := AvailableSinks[config.Type]
	err = s.Init(config)
	if err != nil {
		cclog.ComponentError("SinkManager", "SKIP", s.Name(), "initialization failed:", err.Error())
		return err
	}
	sm.outputs = append(sm.outputs, s)
	sm.config = append(sm.config, config)
	cclog.ComponentDebug("SinkManager", "ADD SINK", s.Name())
	return nil
}

func (sm *sinkManager) Close() {
	select {
	case sm.done <- true:
	default:
	}
	cclog.ComponentDebug("SinkManager", "CLOSE")
}

func New(wg *sync.WaitGroup, sinkConfigFile string) (SinkManager, error) {
	sm := &sinkManager{}
	err := sm.Init(wg, sinkConfigFile)
	if err != nil {
		return nil, err
	}
	return sm, err
}
