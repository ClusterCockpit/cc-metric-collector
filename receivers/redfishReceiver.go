package receivers

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	cclog "github.com/ClusterCockpit/cc-metric-collector/internal/ccLogger"
	lp "github.com/ClusterCockpit/cc-metric-collector/internal/ccMetric"

	// See: https://pkg.go.dev/github.com/stmcginnis/gofish
	"github.com/stmcginnis/gofish"
	"github.com/stmcginnis/gofish/common"
	"github.com/stmcginnis/gofish/redfish"
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

		// Control whether a client verifies the server's certificate
		// (default: true == do not verify server's certificate)
		HttpInsecure bool `json:"http_insecure,omitempty"`
		// Time limit for requests made by this HTTP client (default: 10 s)
		HttpTimeoutString string `json:"http_timeout,omitempty"`
		HttpTimeout       time.Duration

		// Globally disable collection of power, processor or thermal metrics
		DisablePowerMetrics     bool `json:"disable_power_metrics"`
		DisableProcessorMetrics bool `json:"disable_processor_metrics"`
		DisableThermalMetrics   bool `json:"disable_thermal_metrics"`

		// Globally excluded metrics
		ExcludeMetrics []string `json:"exclude_metrics,omitempty"`

		// Client config for each redfish service
		ClientConfigs []struct {
			Hostname *string `json:"hostname"` // Hostname the redfish service belongs to
			Username *string `json:"username"` // User name to authenticate with
			Password *string `json:"password"` // Password to use for authentication
			Endpoint *string `json:"endpoint"` // URL of the redfish service

			// Per client disable collection of power,processor or thermal metrics
			DisablePowerMetrics     bool `json:"disable_power_metrics"`
			DisableProcessorMetrics bool `json:"disable_processor_metrics"`
			DisableThermalMetrics   bool `json:"disable_thermal_metrics"`

			// Per client excluded metrics
			ExcludeMetrics []string `json:"exclude_metrics,omitempty"`

			// is metric excluded globally or per client
			isExcluded map[string](bool)

			gofish gofish.ClientConfig
		} `json:"client_config"`
	}

	done chan bool      // channel to finish / stop redfish receiver
	wg   sync.WaitGroup // wait group for redfish receiver
}

// Start starts the redfish receiver
func (r *RedfishReceiver) Start() {
	cclog.ComponentDebug(r.name, "START")

	// Read redfish thermal metrics
	readThermalMetrics := func(clientConfigIndex int, chassis *redfish.Chassis) error {
		clientConfig := &r.config.ClientConfigs[clientConfigIndex]

		// Skip collection off thermal metrics when disabled by config
		if r.config.DisableThermalMetrics || clientConfig.DisableThermalMetrics {
			return nil
		}

		// Get thermal information for each chassis
		thermal, err := chassis.Thermal()
		if err != nil {
			return fmt.Errorf("readMetrics: chassis.Thermal() failed: %v", err)
		}

		// Skip empty thermal information
		if thermal == nil {
			return nil
		}

		timestamp := time.Now()

		for _, temperature := range thermal.Temperatures {

			// Skip, when temperature metric is excluded
			if clientConfig.isExcluded["temperature"] {
				break
			}

			// Skip all temperatures which are not in enabled state
			if temperature.Status.State != common.EnabledState {
				continue
			}

			tags := map[string]string{
				"hostname": *clientConfig.Hostname,
				"type":     "node",
				// ChassisType shall indicate the physical form factor for the type of chassis
				"chassis_typ": string(chassis.ChassisType),
				// Chassis name
				"chassis_name": chassis.Name,
				// ID uniquely identifies the resource
				"temperature_id": temperature.ID,
				// MemberID shall uniquely identify the member within the collection. For
				// services supporting Redfish v1.6 or higher, this value shall be the
				// zero-based array index.
				"temperature_member_id": temperature.MemberID,
				// PhysicalContext shall be a description of the affected device or region
				// within the chassis to which this temperature measurement applies
				"temperature_physical_context": string(temperature.PhysicalContext),
				// Name
				"temperature_name": temperature.Name,
			}

			// Delete empty tags
			for key, value := range tags {
				if value == "" {
					delete(tags, key)
				}
			}

			// Set meta data tags
			meta := map[string]string{
				"source": r.name,
				"group":  "Temperature",
				"unit":   "degC",
			}

			// ReadingCelsius shall be the current value of the temperature sensor's reading.
			value := temperature.ReadingCelsius

			y, err := lp.New("temperature", tags, meta,
				map[string]interface{}{
					"value": value,
				},
				timestamp)
			if err == nil {
				r.sink <- y
			}
		}

		for _, fan := range thermal.Fans {
			// Skip, when fan_speed metric is excluded
			if clientConfig.isExcluded["fan_speed"] {
				break
			}

			// Skip all fans which are not in enabled state
			if fan.Status.State != common.EnabledState {
				continue
			}

			tags := map[string]string{
				"hostname": *clientConfig.Hostname,
				"type":     "node",
				// ChassisType shall indicate the physical form factor for the type of chassis
				"chassis_typ": string(chassis.ChassisType),
				// Chassis name
				"chassis_name": chassis.Name,
				// ID uniquely identifies the resource
				"fan_id": fan.ID,
				// MemberID shall uniquely identify the member within the collection. For
				// services supporting Redfish v1.6 or higher, this value shall be the
				// zero-based array index.
				"fan_member_id": fan.MemberID,
				// PhysicalContext shall be a description of the affected device or region
				// within the chassis to which this fan is associated
				"fan_physical_context": string(fan.PhysicalContext),
				// Name
				"fan_name": fan.Name,
			}

			// Delete empty tags
			for key, value := range tags {
				if value == "" {
					delete(tags, key)
				}
			}

			// Set meta data tags
			meta := map[string]string{
				"source": r.name,
				"group":  "FanSpeed",
				"unit":   string(fan.ReadingUnits),
			}

			// Reading shall be the current value of the fan sensor's reading
			value := fan.Reading

			y, err := lp.New("fan_speed", tags, meta,
				map[string]interface{}{
					"value": value,
				},
				timestamp)
			if err == nil {
				r.sink <- y
			}
		}

		return nil
	}

	// Read redfish power metrics
	readPowerMetrics := func(clientConfigIndex int, chassis *redfish.Chassis) error {
		clientConfig := &r.config.ClientConfigs[clientConfigIndex]

		// Skip collection off thermal metrics when disabled by config
		if r.config.DisablePowerMetrics || clientConfig.DisablePowerMetrics {
			return nil
		}

		// Get power information for each chassis
		power, err := chassis.Power()
		if err != nil {
			return fmt.Errorf("readMetrics: chassis.Power() failed: %v", err)
		}

		// Skip empty power information
		if power == nil {
			return nil
		}

		timestamp := time.Now()

		// Read min, max and average consumed watts for each power control
		for _, pc := range power.PowerControl {

			// Skip all power controls which are not in enabled state
			if pc.Status.State != common.EnabledState {
				continue
			}

			// Map of collected metrics
			metrics := make(map[string]float32)

			// PowerConsumedWatts shall represent the actual power being consumed (in
			// Watts) by the chassis
			if !clientConfig.isExcluded["consumed_watts"] {
				metrics["consumed_watts"] = pc.PowerConsumedWatts
			}
			// AverageConsumedWatts shall represent the
			// average power level that occurred averaged over the last IntervalInMin
			// minutes.
			if !clientConfig.isExcluded["average_consumed_watts"] {
				metrics["average_consumed_watts"] = pc.PowerMetrics.AverageConsumedWatts
			}
			// MinConsumedWatts shall represent the
			// minimum power level in watts that occurred within the last
			// IntervalInMin minutes.
			if !clientConfig.isExcluded["min_consumed_watts"] {
				metrics["min_consumed_watts"] = pc.PowerMetrics.MinConsumedWatts
			}
			// MaxConsumedWatts shall represent the
			// maximum power level in watts that occurred within the last
			// IntervalInMin minutes
			if !clientConfig.isExcluded["max_consumed_watts"] {
				metrics["max_consumed_watts"] = pc.PowerMetrics.MaxConsumedWatts
			}
			// IntervalInMin shall represent the time interval (or window), in minutes,
			// in which the PowerMetrics properties are measured over.
			// Should be an integer, but some Dell implementations return as a float
			intervalInMin :=
				strconv.FormatFloat(
					float64(pc.PowerMetrics.IntervalInMin), 'f', -1, 32)

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

		return nil
	}

	// Read redfish processor metrics
	// See: https://redfish.dmtf.org/schemas/v1/ProcessorMetrics.json
	readProcessorMetrics := func(clientConfigIndex int, processor *redfish.Processor) error {
		clientConfig := &r.config.ClientConfigs[clientConfigIndex]

		// Skip collection off processor metrics when disabled by config
		if r.config.DisableProcessorMetrics || clientConfig.DisableProcessorMetrics {
			return nil
		}

		timestamp := time.Now()

		URL, _ := url.JoinPath(processor.ODataID, "ProcessorMetrics")
		resp, err := processor.Client.Get(URL)
		if err != nil {
			// Skip non existing URLs
			return nil
		}

		var processorMetrics struct {
			common.Entity
			ODataType   string `json:"@odata.type"`
			ODataEtag   string `json:"@odata.etag"`
			Description string `json:"Description"`
			// This property shall contain the power, in watts, that the processor has consumed.
			ConsumedPowerWatt float32 `json:"ConsumedPowerWatt"`
			// This property shall contain the temperature, in Celsius, of the processor.
			TemperatureCelsius float32 `json:"TemperatureCelsius"`
		}
		err = json.NewDecoder(resp.Body).Decode(&processorMetrics)
		if err != nil {
			return fmt.Errorf("unable to decode JSON for processor metrics: %+w", err)
		}
		processorMetrics.SetClient(processor.Client)

		// Set tags
		tags := map[string]string{
			"hostname": *clientConfig.Hostname,
			"type":     "socket",
			// ProcessorType shall contain the string which identifies the type of processor contained in this Socket
			"processor_typ": string(processor.ProcessorType),
			// Processor name
			"processor_name": processor.Name,
			// ID uniquely identifies the resource
			"processor_id": processor.ID,
		}

		// Delete empty tags
		for key, value := range tags {
			if value == "" {
				delete(tags, key)
			}
		}

		// Set meta data tags
		metaPower := map[string]string{
			"source": r.name,
			"group":  "Energy",
			"unit":   "watts",
		}

		namePower := "consumed_power"

		if !clientConfig.isExcluded[namePower] {
			y, err := lp.New(namePower, tags, metaPower,
				map[string]interface{}{
					"value": processorMetrics.ConsumedPowerWatt,
				},
				timestamp)
			if err == nil {
				r.sink <- y
			}
		}
		// Set meta data tags
		metaThermal := map[string]string{
			"source": r.name,
			"group":  "Temperature",
			"unit":   "degC",
		}

		nameThermal := "temperature"

		if !clientConfig.isExcluded[nameThermal] {
			y, err := lp.New(nameThermal, tags, metaThermal,
				map[string]interface{}{
					"value": processorMetrics.TemperatureCelsius,
				},
				timestamp)
			if err == nil {
				r.sink <- y
			}
		}
		return nil
	}

	// readMetrics reads redfish temperature and power metrics from the endpoint configured in conf
	readMetrics := func(clientConfigIndex int) error {

		// access client config
		clientConfig := &r.config.ClientConfigs[clientConfigIndex]

		// Connect to redfish service
		c, err := gofish.Connect(clientConfig.gofish)
		if err != nil {
			return fmt.Errorf(
				"readMetrics: gofish.Connect({Username: %v, Endpoint: %v, BasicAuth: %v, HttpTimeout: %v, HttpInsecure: %v}) failed: %v",
				clientConfig.gofish.Username,
				clientConfig.gofish.Endpoint,
				clientConfig.gofish.BasicAuth,
				clientConfig.gofish.HTTPClient.Timeout,
				clientConfig.gofish.HTTPClient.Transport.(*http.Transport).TLSClientConfig.InsecureSkipVerify,
				err)
		}
		defer c.Logout()

		// Create a session, when required
		if _, err = c.GetSession(); err != nil {
			c, err = c.CloneWithSession()
			if err != nil {
				return fmt.Errorf("readMetrics: Failed to create a session: %+w", err)
			}
		}

		// Get all chassis managed by this service
		chassis_list, err := c.Service.Chassis()
		if err != nil {
			return fmt.Errorf("readMetrics: c.Service.Chassis() failed: %v", err)
		}

		for _, chassis := range chassis_list {

			err := readThermalMetrics(clientConfigIndex, chassis)
			if err != nil {
				return err
			}

			err = readPowerMetrics(clientConfigIndex, chassis)
			if err != nil {
				return err
			}
		}

		// loop for all computer systems
		systems, err := c.Service.Systems()
		if err != nil {
			return fmt.Errorf("readMetrics: c.Service.Systems() failed: %v", err)
		}
		for _, system := range systems {

			// loop for all processors
			processors, err := system.Processors()
			if err != nil {
				return fmt.Errorf("readMetrics: system.Processors() failed: %v", err)
			}
			for _, processor := range processors {
				err := readProcessorMetrics(clientConfigIndex, processor)
				if err != nil {
					return err
				}
			}
		}

		return nil
	}

	// doReadMetrics read power and temperature metrics for all configure redfish services.
	// To compensate latencies of the Redfish services a fanout is used.
	doReadMetric := func() {

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
					err := readMetrics(clientConfigIndex)
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
			doReadMetric()

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

		// Reuse existing http client
		gofishConfig.HTTPClient = httpClient

		// Is metrics excluded globally or per client
		clientConfig.isExcluded = make(map[string]bool)
		for _, key := range clientConfig.ExcludeMetrics {
			clientConfig.isExcluded[key] = true
		}
		for _, key := range r.config.ExcludeMetrics {
			clientConfig.isExcluded[key] = true
		}
	}

	return r, nil
}
