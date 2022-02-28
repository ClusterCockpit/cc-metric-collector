package receivers

import (
	"encoding/json"
	"fmt"

	cclog "github.com/ClusterCockpit/cc-metric-collector/internal/ccLogger"
)

// SampleReceiver configuration: receiver type, listen address, port
type SampleReceiverConfig struct {
	Type string `json:"type"`
	Addr string `json:"address"`
	Port string `json:"port"`
}

type SampleReceiver struct {
	receiver
	config SampleReceiverConfig

	// Storage for static information
	meta map[string]string
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
	// r.wg.Add(1)
	// go func() {
	// 	for {
	// 		select {
	// 		case <-r.done:
	// 			r.wg.Done()
	// 			return
	// 		}
	// 	}
	// 	r.wg.Done()
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

	// Set static information
	r.meta = map[string]string{"source": r.name}

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

	// Check that all required fields in the configuration are set
	// Use 'if len(r.config.Option) > 0' for strings

	return r, nil
}
