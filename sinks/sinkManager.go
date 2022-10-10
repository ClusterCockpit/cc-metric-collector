package sinks

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"

	cclog "github.com/ClusterCockpit/cc-metric-collector/pkg/ccLogger"
	lp "github.com/ClusterCockpit/cc-metric-collector/pkg/ccMetric"
)

const SINK_MAX_FORWARD = 50

// Map of all available sinks
var AvailableSinks = map[string]func(name string, config json.RawMessage) (Sink, error){
	"ganglia":     NewGangliaSink,
	"libganglia":  NewLibgangliaSink,
	"stdout":      NewStdoutSink,
	"nats":        NewNatsSink,
	"influxdb":    NewInfluxSink,
	"influxasync": NewInfluxAsyncSink,
	"http":        NewHttpSink,
}

// Metric collector manager data structure
type sinkManager struct {
	input      chan lp.CCMetric // input channel
	done       chan bool        // channel to finish / stop metric sink manager
	wg         *sync.WaitGroup  // wait group for all goroutines in cc-metric-collector
	sinks      map[string]Sink  // Mapping sink name to sink
	maxForward int              // number of metrics to write maximally in one iteration
}

// Sink manager access functions
type SinkManager interface {
	Init(wg *sync.WaitGroup, sinkConfigFile string) error
	AddInput(input chan lp.CCMetric)
	AddOutput(name string, config json.RawMessage) error
	Start()
	Close()
}

// Init initializes the sink manager by:
// * Reading its configuration file
// * Adding the configured sinks and providing them with the corresponding config
func (sm *sinkManager) Init(wg *sync.WaitGroup, sinkConfigFile string) error {
	sm.input = nil
	sm.done = make(chan bool)
	sm.wg = wg
	sm.sinks = make(map[string]Sink, 0)
	sm.maxForward = SINK_MAX_FORWARD

	if len(sinkConfigFile) == 0 {
		return nil
	}

	// Read sink config file
	configFile, err := os.Open(sinkConfigFile)
	if err != nil {
		cclog.ComponentError("SinkManager", err.Error())
		return err
	}
	defer configFile.Close()

	// Parse config
	jsonParser := json.NewDecoder(configFile)
	var rawConfigs map[string]json.RawMessage
	err = jsonParser.Decode(&rawConfigs)
	if err != nil {
		cclog.ComponentError("SinkManager", err.Error())
		return err
	}

	// Start sinks
	for name, raw := range rawConfigs {
		err = sm.AddOutput(name, raw)
		if err != nil {
			cclog.ComponentError("SinkManager", err)
			continue
		}
	}

	// Check that at least one sink is running
	if !(len(sm.sinks) > 0) {
		cclog.ComponentError("SinkManager", "Found no usable sinks")
		return fmt.Errorf("Found no usable sinks")
	}

	return nil
}

// Start starts the sink managers background task, which
// distributes received metrics to the sinks
func (sm *sinkManager) Start() {
	sm.wg.Add(1)
	go func() {
		defer sm.wg.Done()

		// Sink manager is done
		done := func() {
			for _, s := range sm.sinks {
				s.Close()
			}

			close(sm.done)
			cclog.ComponentDebug("SinkManager", "DONE")
		}

		toTheSinks := func(p lp.CCMetric) {
			// Send received metric to all outputs
			cclog.ComponentDebug("SinkManager", "WRITE", p)
			for _, s := range sm.sinks {
				if err := s.Write(p); err != nil {
					cclog.ComponentError("SinkManager", "WRITE", s.Name(), "write failed:", err.Error())
				}
			}
		}

		for {
			select {
			case <-sm.done:
				done()
				return

			case p := <-sm.input:
				toTheSinks(p)
				for i := 0; len(sm.input) > 0 && i < sm.maxForward; i++ {
					p := <-sm.input
					toTheSinks(p)
				}
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

func (sm *sinkManager) AddOutput(name string, rawConfig json.RawMessage) error {
	var err error
	var sinkConfig defaultSinkConfig
	if len(rawConfig) > 0 {
		err := json.Unmarshal(rawConfig, &sinkConfig)
		if err != nil {
			return err
		}
	}
	if _, found := AvailableSinks[sinkConfig.Type]; !found {
		cclog.ComponentError("SinkManager", "SKIP", name, "unknown sink:", sinkConfig.Type)
		return err
	}
	s, err := AvailableSinks[sinkConfig.Type](name, rawConfig)
	if err != nil {
		cclog.ComponentError("SinkManager", "SKIP", s.Name(), "initialization failed:", err.Error())
		return err
	}
	sm.sinks[name] = s
	cclog.ComponentDebug("SinkManager", "ADD SINK", s.Name(), "with name", fmt.Sprintf("'%s'", name))
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
	sm := new(sinkManager)
	err := sm.Init(wg, sinkConfigFile)
	if err != nil {
		return nil, err
	}
	return sm, err
}
