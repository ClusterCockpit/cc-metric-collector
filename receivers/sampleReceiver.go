package receivers

import (
	"encoding/json"
	"fmt"

	cclog "github.com/ClusterCockpit/cc-metric-collector/internal/ccLogger"
)

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

func (r *SampleReceiver) Close() {
	cclog.ComponentDebug(r.name, "CLOSE")

	// Close server like http.Shutdown()

	// in case of own go routine, send the signal and wait
	// r.done <- true
	// r.wg.Wait()
}

func NewSampleReceiver(name string, config json.RawMessage) (Receiver, error) {
	r := new(SampleReceiver)
	r.name = fmt.Sprintf("HttpReceiver(%s)", name)

	// Set static information
	r.meta = map[string]string{"source": r.name}

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
