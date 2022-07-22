package receivers

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
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
		Type string `json:"type"`

		// Maximum number of simultaneous redfish connections (default: 64)
		Fanout int `json:"fanout,omitempty"`
		// How often the redfish power metrics should be read and send to the sink (default: 30 s)
		IntervalString string `json:"interval,omitempty"`
		Interval       time.Duration

		// Control whether a client verifies the server's certificate (default: true)
		HttpInsecure bool `json:"http_insecure,omitempty"`
		// Time limit for requests made by this HTTP client (default: 10 s)
		HttpTimeoutString string `json:"http_timeout,omitempty"`
		HttpTimeout       time.Duration

		// Client config for each redfish service
		ClientConfigs []struct {
			Hostname       *string  `json:"hostname"`
			Username       *string  `json:"username"`
			Password       *string  `json:"password"`
			Endpoint       *string  `json:"endpoint"`
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

	// readPowerMetric reads redfish power metric from the endpoint configured in conf
	readPowerMetric := func(clientConfigIndex int) error {

		clientConfig := &r.config.ClientConfigs[clientConfigIndex]

		// Connect to redfish service
		c, err := gofish.Connect(clientConfig.gofish)
		if err != nil {
			return fmt.Errorf(
				"readPowerMetric: gofish.Connect({Username: %v, Endpoint: %v, BasicAuth: %v, HttpTimeout: %v, HttpInsecure: %v}) failed: %v",
				clientConfig.gofish.Username,
				clientConfig.gofish.Endpoint,
				clientConfig.gofish.BasicAuth,
				clientConfig.gofish.HTTPClient.Timeout,
				clientConfig.gofish.HTTPClient.Transport.(*http.Transport).TLSClientConfig.InsecureSkipVerify,
				err)
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
			if power == nil {
				continue
			}

			// Read min, max and average consumed watts for each power control
			for _, pc := range power.PowerControl {

				// Map of collected metrics
				metrics := map[string]float32{
					// PowerConsumedWatts shall represent the actual power being consumed (in
					// Watts) by the chassis
					"consumed_watts": pc.PowerConsumedWatts,
					// AverageConsumedWatts shall represent the
					// average power level that occurred averaged over the last IntervalInMin
					// minutes.
					"average_consumed_watts": pc.PowerMetrics.AverageConsumedWatts,
					// MinConsumedWatts shall represent the
					// minimum power level in watts that occurred within the last
					// IntervalInMin minutes.
					"min_consumed_watts": pc.PowerMetrics.MinConsumedWatts,
					// MaxConsumedWatts shall represent the
					// maximum power level in watts that occurred within the last
					// IntervalInMin minutes
					"max_consumed_watts": pc.PowerMetrics.MaxConsumedWatts,
				}
				intervalInMin := strconv.FormatFloat(float64(pc.PowerMetrics.IntervalInMin), 'f', -1, 32)

				// Metrics to exclude
				for _, key := range clientConfig.ExcludeMetrics {
					delete(metrics, key)
				}

				// Set tags
				tags := map[string]string{
					"hostname": *clientConfig.Hostname,
					"type":     "node",
					// ChassisType shall indicate the physical form factor for the type of chassis
					"chassis_typ": string(chassis.ChassisType),
					// Chassis name
					"chassis_name": chassis.Name,
					// ID uniquely identifies the resource
					"power_control_id": pc.ID,
					// MemberID shall uniquely identify the member within the collection. For
					// services supporting Redfish v1.6 or higher, this value shall be the
					// zero-based array index.
					"power_control_member_id": pc.MemberID,
					// PhysicalContext shall be a description of the affected device(s) or region
					// within the chassis to which this power control applies.
					"power_control_physical_context": string(pc.PhysicalContext),
					// Name
					"power_control_name": pc.Name,
				}

				// Delete empty tags
				for key, value := range tags {
					if value == "" {
						delete(tags, key)
					}
				}

				// Set meta data tags
				meta := map[string]string{
					"source":              r.name,
					"group":               "Energy",
					"interval_in_minutes": intervalInMin,
					"unit":                "watts",
				}

				// Delete empty meta data tags
				for key, value := range meta {
					if value == "" {
						delete(meta, key)
					}
				}

				for name, value := range metrics {

					y, err := lp.New(name, tags, meta,
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
		ticker := time.NewTicker(r.config.Interval)
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
	r.config.IntervalString = "30s"
	r.config.HttpTimeoutString = "10s"
	r.config.HttpInsecure = true

	// Read the redfish receiver specific JSON config
	if len(config) > 0 {
		err := json.Unmarshal(config, &r.config)
		if err != nil {
			cclog.ComponentError(r.name, "Error reading config:", err.Error())
			return nil, err
		}
	}

	// interval duration
	var err error
	r.config.Interval, err = time.ParseDuration(r.config.IntervalString)
	if err != nil {
		err := fmt.Errorf(
			"Failed to parse duration string interval='%s': %w",
			r.config.IntervalString,
			err,
		)
		cclog.Error(r.name, err)
		return nil, err
	}

	// HTTP timeout duration
	r.config.HttpTimeout, err = time.ParseDuration(r.config.HttpTimeoutString)
	if err != nil {
		err := fmt.Errorf(
			"Failed to parse duration string http_timeout='%s': %w",
			r.config.HttpTimeoutString,
			err,
		)
		cclog.Error(r.name, err)
		return nil, err
	}

	// Create new http client
	customTransport := http.DefaultTransport.(*http.Transport).Clone()
	customTransport.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: r.config.HttpInsecure,
	}
	httpClient := &http.Client{
		Timeout:   r.config.HttpTimeout,
		Transport: customTransport,
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

		gofishConfig.HTTPClient = httpClient
	}

	return r, nil
}
