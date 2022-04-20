package receivers

import (
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	cclog "github.com/ClusterCockpit/cc-metric-collector/internal/ccLogger"
	lp "github.com/ClusterCockpit/cc-metric-collector/internal/ccMetric"

	// See: https://pkg.go.dev/github.com/stmcginnis/gofish
	"github.com/stmcginnis/gofish"
)

// RedfishReceiver configuration:
type RedfishReceiver struct {
	receiver
	config struct {
		Type     string `json:"type"`
		Fanout   int    `json:"fanout,omitempty"`   // Default fanout: 64
		Interval int    `json:"interval,omitempty"` // Default interval: 30s

		// Client config for each redfish service
		ClientConfigs []struct {
			Hostname       *string  `json:"hostname"`
			Username       *string  `json:"username"`
			Password       *string  `json:"password"`
			Endpoint       *string  `json:"endpoint"`
			Insecure       *bool    `json:"insecure,omitempty"`
			ExcludeMetrics []string `json:"exclude_metrics,omitempty"`
			gofish         gofish.ClientConfig
		} `json:"client_config"`
	}

	done chan bool      // channel to finish / stop redfish receiver
	wg   sync.WaitGroup // wait group for redfish receiver
}

// Start starts the redfish receiver
func (r *RedfishReceiver) Start() {
	cclog.ComponentDebug(r.name, "START")

	// readPowerMetric reads readfish power metric from the endpoint configured in conf
	readPowerMetric := func(clientConfigIndex int) error {

		clientConfig := &r.config.ClientConfigs[clientConfigIndex]

		// Connect to redfish service
		c, err := gofish.Connect(clientConfig.gofish)
		if err != nil {
			c := struct {
				Username  string
				Endpoint  string
				BasicAuth bool
				Insecure  bool
			}{
				Username:  clientConfig.gofish.Username,
				Endpoint:  clientConfig.gofish.Endpoint,
				BasicAuth: clientConfig.gofish.BasicAuth,
				Insecure:  clientConfig.gofish.Insecure,
			}
			return fmt.Errorf("readPowerMetric: gofish.Connect(%+v) failed: %v", c, err)
		}
		defer c.Logout()

		// Get all chassis managed by this service
		chassis_list, err := c.Service.Chassis()
		if err != nil {
			return fmt.Errorf("readPowerMetric: c.Service.Chassis() failed: %v", err)
		}

		for _, chassis := range chassis_list {
			timestamp := time.Now()

			// Get power information for each chassis
			power, err := chassis.Power()
			if err != nil {
				return fmt.Errorf("readPowerMetric: chassis.Power() failed: %v", err)
			}

			// Read min, max and average consumed watts for each power control
			for _, pc := range power.PowerControl {

				// Map of collected metrics
				metrics := map[string]float32{
					"average_consumed_watts": pc.PowerMetrics.AverageConsumedWatts,
					"min_consumed_watts":     pc.PowerMetrics.MinConsumedWatts,
					"max_consumed_watts":     pc.PowerMetrics.MaxConsumedWatts,
				}
				intervalInMin := strconv.FormatFloat(float64(pc.PowerMetrics.IntervalInMin), 'f', -1, 32)

				// Metrics to exclude
				for _, key := range clientConfig.ExcludeMetrics {
					delete(metrics, key)
				}

				for name, value := range metrics {
					y, err := lp.New(
						name,
						map[string]string{
							"hostname":           *clientConfig.Hostname,
							"type":               "node",
							"power_control_name": pc.Name,
						},
						map[string]string{
							"source":              r.name,
							"group":               "Energy",
							"interval_in_minutes": intervalInMin,
							"unit":                "watts",
						},
						map[string]interface{}{
							"value": value,
						},
						timestamp)
					if err == nil {
						r.sink <- y
					}
				}
			}
		}

		return nil
	}

	// doReadPowerMetric read power metrics for all configure redfish services.
	// To compensate latencies of the Redfish services a fanout is used.
	doReadPowerMetric := func() {

		// Compute fanout to use
		realFanout := r.config.Fanout
		if len(r.config.ClientConfigs) < realFanout {
			realFanout = len(r.config.ClientConfigs)
		}

		// Create wait group and input channel for workers
		var workerWaitGroup sync.WaitGroup
		workerInput := make(chan int, realFanout)

		// Create worker go routines
		for i := 0; i < realFanout; i++ {
			// Increment worker wait group counter
			workerWaitGroup.Add(1)
			go func() {
				// Decrement worker wait group counter
				defer workerWaitGroup.Done()

				// Read power metrics for each client config
				for clientConfigIndex := range workerInput {
					err := readPowerMetric(clientConfigIndex)
					if err != nil {
						cclog.ComponentError(r.name, err)
					}
				}
			}()
		}

		// Distribute client configs to workers
		for i := range r.config.ClientConfigs {
			// Check done channel status
			select {
			case workerInput <- i:
			case <-r.done:
				// process done event
				// Stop workers, clear channel and wait for all workers to finish
				close(workerInput)
				for range workerInput {
				}
				workerWaitGroup.Wait()
				return
			}
		}

		// Stop workers and wait for all workers to finish
		close(workerInput)
		workerWaitGroup.Wait()
	}

	// Start redfish receiver
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()

		// Create ticker
		ticker := time.NewTicker(time.Duration(r.config.Interval) * time.Second)
		defer ticker.Stop()

		for {
			doReadPowerMetric()

			select {
			case <-ticker.C:
				// process ticker event -> continue
				continue
			case <-r.done:
				// process done event
				return
			}
		}
	}()

	cclog.ComponentDebug(r.name, "STARTED")
}

// Close redfish receiver
func (r *RedfishReceiver) Close() {
	cclog.ComponentDebug(r.name, "CLOSE")

	// Send the signal and wait
	close(r.done)
	r.wg.Wait()

	cclog.ComponentDebug(r.name, "DONE")
}

// New function to create a new instance of the receiver
// Initialize the receiver by giving it a name and reading in the config JSON
func NewRedfishReceiver(name string, config json.RawMessage) (Receiver, error) {
	r := new(RedfishReceiver)

	// Set name
	r.name = fmt.Sprintf("RedfishReceiver(%s)", name)

	// Create done channel
	r.done = make(chan bool)

	// Set defaults in r.config
	// Allow overwriting these defaults by reading config JSON
	r.config.Fanout = 64
	r.config.Interval = 30

	// Read the redfish receiver specific JSON config
	if len(config) > 0 {
		err := json.Unmarshal(config, &r.config)
		if err != nil {
			cclog.ComponentError(r.name, "Error reading config:", err.Error())
			return nil, err
		}
	}

	// Create gofish client config
	for i := range r.config.ClientConfigs {
		clientConfig := &r.config.ClientConfigs[i]
		gofishConfig := &clientConfig.gofish

		if clientConfig.Hostname == nil {
			err := fmt.Errorf("client config number %v requires hostname", i)
			cclog.ComponentError(r.name, err)
			return nil, err
		}

		if clientConfig.Endpoint == nil {
			err := fmt.Errorf("client config number %v requires endpoint", i)
			cclog.ComponentError(r.name, err)
			return nil, err
		}
		gofishConfig.Endpoint = *clientConfig.Endpoint

		if clientConfig.Username == nil {
			err := fmt.Errorf("client config number %v requires username", i)
			cclog.ComponentError(r.name, err)
			return nil, err
		}
		gofishConfig.Username = *clientConfig.Username

		if clientConfig.Password == nil {
			err := fmt.Errorf("client config number %v requires password", i)
			cclog.ComponentError(r.name, err)
			return nil, err
		}
		gofishConfig.Password = *clientConfig.Password

		gofishConfig.Insecure = true
		if clientConfig.Insecure != nil {
			gofishConfig.Insecure = *clientConfig.Insecure
		}
	}

	return r, nil
}
