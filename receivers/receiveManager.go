package receivers

import (
	"encoding/json"
	"os"
	"sync"

	cclog "github.com/ClusterCockpit/cc-metric-collector/internal/ccLogger"
	lp "github.com/ClusterCockpit/cc-metric-collector/internal/ccMetric"
)

var AvailableReceivers = map[string]Receiver{
	"nats": &NatsReceiver{},
}

type receiveManager struct {
	inputs []Receiver
	output chan lp.CCMetric
	done   chan bool
	wg     *sync.WaitGroup
	config []json.RawMessage
}

type ReceiveManager interface {
	Init(wg *sync.WaitGroup, receiverConfigFile string) error
	AddInput(name string, rawConfig json.RawMessage) error
	AddOutput(output chan lp.CCMetric)
	Start()
	Close()
}

func (rm *receiveManager) Init(wg *sync.WaitGroup, receiverConfigFile string) error {
	rm.inputs = make([]Receiver, 0)
	rm.output = nil
	rm.done = make(chan bool)
	rm.wg = wg
	rm.config = make([]json.RawMessage, 0)
	configFile, err := os.Open(receiverConfigFile)
	if err != nil {
		cclog.ComponentError("ReceiveManager", err.Error())
		return err
	}
	defer configFile.Close()
	jsonParser := json.NewDecoder(configFile)
	var rawConfigs map[string]json.RawMessage
	err = jsonParser.Decode(&rawConfigs)
	if err != nil {
		cclog.ComponentError("ReceiveManager", err.Error())
		return err
	}
	for name, raw := range rawConfigs {
		rm.AddInput(name, raw)
	}
	return nil
}

func (rm *receiveManager) Start() {
	rm.wg.Add(1)

	for _, r := range rm.inputs {
		cclog.ComponentDebug("ReceiveManager", "START", r.Name())
		r.Start()
	}
	cclog.ComponentDebug("ReceiveManager", "STARTED")
}

func (rm *receiveManager) AddInput(name string, rawConfig json.RawMessage) error {
	var config defaultReceiverConfig
	err := json.Unmarshal(rawConfig, &config)
	if err != nil {
		cclog.ComponentError("ReceiveManager", "SKIP", config.Type, "JSON config error:", err.Error())
		return err
	}
	if _, found := AvailableReceivers[config.Type]; !found {
		cclog.ComponentError("ReceiveManager", "SKIP", config.Type, "unknown receiver:", err.Error())
		return err
	}
	r := AvailableReceivers[config.Type]
	err = r.Init(name, rawConfig)
	if err != nil {
		cclog.ComponentError("ReceiveManager", "SKIP", r.Name(), "initialization failed:", err.Error())
		return err
	}
	rm.inputs = append(rm.inputs, r)
	rm.config = append(rm.config, rawConfig)
	cclog.ComponentDebug("ReceiveManager", "ADD RECEIVER", r.Name())
	return nil
}

func (rm *receiveManager) AddOutput(output chan lp.CCMetric) {
	rm.output = output
	for _, r := range rm.inputs {
		r.SetSink(rm.output)
	}
}

func (rm *receiveManager) Close() {
	for _, r := range rm.inputs {
		cclog.ComponentDebug("ReceiveManager", "CLOSE", r.Name())
		r.Close()
	}
	rm.wg.Done()
	cclog.ComponentDebug("ReceiveManager", "CLOSE")
}

func New(wg *sync.WaitGroup, receiverConfigFile string) (ReceiveManager, error) {
	r := &receiveManager{}
	err := r.Init(wg, receiverConfigFile)
	if err != nil {
		return nil, err
	}
	return r, err
}
