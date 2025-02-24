package collectors

import (
	"encoding/json"
	"sync"
	"time"

	lp "github.com/ClusterCockpit/cc-lib/ccMessage"
	cclog "github.com/ClusterCockpit/cc-metric-collector/pkg/ccLogger"
)

// These are the fields we read from the JSON configuration
type SampleTimerCollectorConfig struct {
	Interval string `json:"interval"`
}

// This contains all variables we need during execution and the variables
// defined by metricCollector (name, init, ...)
type SampleTimerCollector struct {
	metricCollector
	wg       sync.WaitGroup             // sync group for management
	done     chan bool                  // channel for management
	meta     map[string]string          // default meta information
	tags     map[string]string          // default tags
	config   SampleTimerCollectorConfig // the configuration structure
	interval time.Duration              // the interval parsed from configuration
	ticker   *time.Ticker               // own timer
	output   chan lp.CCMessage          // own internal output channel
}

func (m *SampleTimerCollector) Init(name string, config json.RawMessage) error {
	var err error = nil
	// Always set the name early in Init() to use it in cclog.Component* functions
	m.name = "SampleTimerCollector"
	// This is for later use, also call it early
	m.setup()
	// Define meta information sent with each metric
	// (Can also be dynamic or this is the basic set with extension through AddMeta())
	m.meta = map[string]string{"source": m.name, "group": "SAMPLE"}
	// Define tags sent with each metric
	// The 'type' tag is always needed, it defines the granularity of the metric
	// node -> whole system
	// socket -> CPU socket (requires socket ID as 'type-id' tag)
	// cpu -> single CPU hardware thread (requires cpu ID as 'type-id' tag)
	m.tags = map[string]string{"type": "node"}
	// Read in the JSON configuration
	if len(config) > 0 {
		err = json.Unmarshal(config, &m.config)
		if err != nil {
			cclog.ComponentError(m.name, "Error reading config:", err.Error())
			return err
		}
	}
	// Parse the read interval duration
	m.interval, err = time.ParseDuration(m.config.Interval)
	if err != nil {
		cclog.ComponentError(m.name, "Error parsing interval:", err.Error())
		return err
	}

	// Storage for output channel
	m.output = nil
	// Management channel for the timer function.
	m.done = make(chan bool)
	// Create the own ticker
	m.ticker = time.NewTicker(m.interval)

	// Start the timer loop with return functionality by sending 'true' to the done channel
	m.wg.Add(1)
	go func() {
		select {
		case <-m.done:
			// Exit the timer loop
			cclog.ComponentDebug(m.name, "Closing...")
			m.wg.Done()
			return
		case timestamp := <-m.ticker.C:
			// This is executed every timer tick but we have to wait until the first
			// Read() to get the output channel
			if m.output != nil {
				m.ReadMetrics(timestamp)
			}
		}
	}()

	// Set this flag only if everything is initialized properly, all required files exist, ...
	m.init = true
	return err
}

// This function is called at each interval timer tick
func (m *SampleTimerCollector) ReadMetrics(timestamp time.Time) {
	// Create a sample metric

	value := 1.0

	// If you want to measure something for a specific amount of time, use interval
	// start := readState()
	// time.Sleep(interval)
	// stop := readState()
	// value = (stop - start) / interval.Seconds()

	y, err := lp.NewMessage("sample_metric", m.tags, m.meta, map[string]interface{}{"value": value}, timestamp)
	if err == nil && m.output != nil {
		// Send it to output channel if we have a valid channel
		m.output <- y
	}
}

func (m *SampleTimerCollector) Read(interval time.Duration, output chan lp.CCMessage) {
	// Capture output channel
	m.output = output
}

func (m *SampleTimerCollector) Close() {
	// Send signal to the timer loop to stop it
	m.done <- true
	// Wait until the timer loop is done
	m.wg.Wait()
	// Unset flag
	m.init = false
}
