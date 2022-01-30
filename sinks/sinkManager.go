package sinks

import (
	"encoding/json"
	"os"
	"sync"

	cclog "github.com/ClusterCockpit/cc-metric-collector/internal/ccLogger"
	lp "github.com/ClusterCockpit/cc-metric-collector/internal/ccMetric"
)

// Map of all available sinks
var AvailableSinks = map[string]Sink{
	"influxdb": new(InfluxSink),
	"stdout":   new(StdoutSink),
	"nats":     new(NatsSink),
	"http":     new(HttpSink),
	"ganglia":  new(GangliaSink),
}

// Metric collector manager data structure
type sinkManager struct {
	input   chan lp.CCMetric // input channel
	outputs []Sink           // List of sinks to use
	done    chan bool        // channel to finish / stop metric sink manager
	wg      *sync.WaitGroup  // wait group for all goroutines in cc-metric-collector
	config  []sinkConfig     // json encoded config for sink manager
}

// Sink manager access functions
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

	// Read sink config file
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
	batchcount := 20

	sm.wg.Add(1)
	go func() {
		defer sm.wg.Done()

		// Sink manager is done
		done := func() {
			for _, s := range sm.outputs {
				s.Flush()
				s.Close()
			}

			close(sm.done)
			cclog.ComponentDebug("SinkManager", "DONE")
		}

		for {
			select {
			case <-sm.done:
				done()
				return

			case p := <-sm.input:
				// Send received metric to all outputs
				cclog.ComponentDebug("SinkManager", "WRITE", p)
				for _, s := range sm.outputs {
					s.Write(p)
				}

				// Flush all outputs
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

	// Sink manager is started
	cclog.ComponentDebug("SinkManager", "STARTED")
}

// AddInput adds the input channel to the sink manager
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

// Close finishes / stops the sink manager
func (sm *sinkManager) Close() {
	cclog.ComponentDebug("SinkManager", "CLOSE")
	sm.done <- true
	// wait for close of channel sm.done
	<-sm.done
}

// New creates a new initialized sink manager
func New(wg *sync.WaitGroup, sinkConfigFile string) (SinkManager, error) {
	sm := &sinkManager{}
	err := sm.Init(wg, sinkConfigFile)
	if err != nil {
		return nil, err
	}
	return sm, err
}
