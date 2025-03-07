package receivers

import (
	"encoding/json"
	"fmt"

	cclog "github.com/ClusterCockpit/cc-metric-collector/pkg/ccLogger"
	mp "github.com/ClusterCockpit/cc-lib/messageProcessor"
)

// SampleReceiver configuration: receiver type, listen address, port
// The defaultReceiverConfig contains the keys 'type' and 'process_messages'
type SampleReceiverConfig struct {
	defaultReceiverConfig
	Addr string `json:"address"`
	Port string `json:"port"`
}

type SampleReceiver struct {
	receiver
	config SampleReceiverConfig

	// Storage for static information
	// Use in case of own go routine
	// done chan bool
	// wg   sync.WaitGroup
}

// Implement functions required for Receiver interface
// Start(), Close()
// See: metricReceiver.go

func (r *SampleReceiver) Start() {
	cclog.ComponentDebug(r.name, "START")

	// Start server process like http.ListenAndServe()

	// or use own go routine but always make sure it exits
	// as soon as it gets the signal of the r.done channel
	//
	// r.done = make(chan bool)
	// r.wg.Add(1)
	// go func() {
	//      defer r.wg.Done()
	//
	//      // Create ticker
	//      ticker := time.NewTicker(30 * time.Second)
	//      defer ticker.Stop()
	//
	//      for {
	//          readMetric()
	//          select {
	//          case <-ticker.C:
	//              // process ticker event -> continue
	//              continue
	//          case <-r.done:
	//              return
	//          }
	//      }
	// }()
}

// Close receiver: close network connection, close files, close libraries, ...
func (r *SampleReceiver) Close() {
	cclog.ComponentDebug(r.name, "CLOSE")

	// Close server like http.Shutdown()

	// in case of own go routine, send the signal and wait
	// r.done <- true
	// r.wg.Wait()
}

// New function to create a new instance of the receiver
// Initialize the receiver by giving it a name and reading in the config JSON
func NewSampleReceiver(name string, config json.RawMessage) (Receiver, error) {
	r := new(SampleReceiver)

	// Set name of SampleReceiver
	// The name should be chosen in such a way that different instances of SampleReceiver can be distinguished
	r.name = fmt.Sprintf("SampleReceiver(%s)", name)

	// create new message processor
	p, err := mp.NewMessageProcessor()
	if err != nil {
		cclog.ComponentError(r.name, "Initialization of message processor failed:", err.Error())
		return nil, fmt.Errorf("initialization of message processor failed: %v", err.Error())
	}
	r.mp = p
	// Set static information
	err = r.mp.AddAddMetaByCondition("true", "source", r.name)
	if err != nil {
		cclog.ComponentError(r.name, fmt.Sprintf("Failed to add static information source=%s:", r.name), err.Error())
		return nil, fmt.Errorf("failed to add static information source=%s: %v", r.name, err.Error())
	}

	// Set defaults in r.config
	// Allow overwriting these defaults by reading config JSON

	// Read the sample receiver specific JSON config
	if len(config) > 0 {
		err := json.Unmarshal(config, &r.config)
		if err != nil {
			cclog.ComponentError(r.name, "Error reading config:", err.Error())
			return nil, err
		}
	}

	// Add message processor config
	if len(r.config.MessageProcessor) > 0 {
		err = r.mp.FromConfigJSON(r.config.MessageProcessor)
		if err != nil {
			cclog.ComponentError(r.name, "Failed parsing JSON for message processor:", err.Error())
			return nil, fmt.Errorf("failed parsing JSON for message processor: %v", err.Error())
		}
	}

	// Check that all required fields in the configuration are set
	// Use 'if len(r.config.Option) > 0' for strings

	return r, nil
}
