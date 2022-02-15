package metricRouter

import (
	"encoding/json"
	"os"
	"strings"
	"sync"
	"time"

	cclog "github.com/ClusterCockpit/cc-metric-collector/internal/ccLogger"

	lp "github.com/ClusterCockpit/cc-metric-collector/internal/ccMetric"
	agg "github.com/ClusterCockpit/cc-metric-collector/internal/metricAggregator"
	mct "github.com/ClusterCockpit/cc-metric-collector/internal/multiChanTicker"
)

const ROUTER_MAX_FORWARD = 50

// Metric router tag configuration
type metricRouterTagConfig struct {
	Key       string `json:"key"`   // Tag name
	Value     string `json:"value"` // Tag value
	Condition string `json:"if"`    // Condition for adding or removing corresponding tag
}

// Metric router configuration
type metricRouterConfig struct {
	AddTags           []metricRouterTagConfig              `json:"add_tags"`            // List of tags that are added when the condition is met
	DelTags           []metricRouterTagConfig              `json:"delete_tags"`         // List of tags that are removed when the condition is met
	IntervalAgg       []agg.MetricAggregatorIntervalConfig `json:"interval_aggregates"` // List of aggregation function processed at the end of an interval
	DropMetrics       []string                             `json:"drop_metrics"`        // List of metric names to drop. For fine-grained dropping use drop_metrics_if
	DropMetricsIf     []string                             `json:"drop_metrics_if"`     // List of evaluatable terms to drop metrics
	RenameMetrics     map[string]string                    `json:"rename_metrics"`      // Map to rename metric name from key to value
	IntervalStamp     bool                                 `json:"interval_timestamp"`  // Update timestamp periodically by ticker each interval?
	NumCacheIntervals int                                  `json:"num_cache_intervals"` // Number of intervals of cached metrics for evaluation
	dropMetrics       map[string]bool                      // Internal map for O(1) lookup
}

// Metric router data structure
type metricRouter struct {
	hostname    string              // Hostname used in tags
	coll_input  chan lp.CCMetric    // Input channel from CollectorManager
	recv_input  chan lp.CCMetric    // Input channel from ReceiveManager
	cache_input chan lp.CCMetric    // Input channel from MetricCache
	outputs     []chan lp.CCMetric  // List of all output channels
	done        chan bool           // channel to finish / stop metric router
	wg          *sync.WaitGroup     // wait group for all goroutines in cc-metric-collector
	timestamp   time.Time           // timestamp periodically updated by ticker each interval
	timerdone   chan bool           // channel to finish / stop timestamp updater
	ticker      mct.MultiChanTicker // periodically ticking once each interval
	config      metricRouterConfig  // json encoded config for metric router
	cache       MetricCache         // pointer to MetricCache
	cachewg     sync.WaitGroup      // wait group for MetricCache
	maxForward  int                 // number of metrics to forward maximally in one iteration
}

// MetricRouter access functions
type MetricRouter interface {
	Init(ticker mct.MultiChanTicker, wg *sync.WaitGroup, routerConfigFile string) error
	AddCollectorInput(input chan lp.CCMetric)
	AddReceiverInput(input chan lp.CCMetric)
	AddOutput(output chan lp.CCMetric)
	Start()
	Close()
}

// Init initializes a metric router by setting up:
// * input and output channels
// * done channel
// * wait group synchronization (from variable wg)
// * ticker (from variable ticker)
// * configuration (read from config file in variable routerConfigFile)
func (r *metricRouter) Init(ticker mct.MultiChanTicker, wg *sync.WaitGroup, routerConfigFile string) error {
	r.outputs = make([]chan lp.CCMetric, 0)
	r.done = make(chan bool)
	r.cache_input = make(chan lp.CCMetric)
	r.wg = wg
	r.ticker = ticker
	r.maxForward = ROUTER_MAX_FORWARD

	// Set hostname
	hostname, err := os.Hostname()
	if err != nil {
		cclog.Error(err.Error())
		return err
	}
	// Drop domain part of host name
	r.hostname = strings.SplitN(hostname, `.`, 2)[0]

	// Read metric router config file
	configFile, err := os.Open(routerConfigFile)
	if err != nil {
		cclog.ComponentError("MetricRouter", err.Error())
		return err
	}
	defer configFile.Close()
	jsonParser := json.NewDecoder(configFile)
	err = jsonParser.Decode(&r.config)
	if err != nil {
		cclog.ComponentError("MetricRouter", err.Error())
		return err
	}
	if r.config.NumCacheIntervals > 0 {
		r.cache, err = NewCache(r.cache_input, r.ticker, &r.cachewg, r.config.NumCacheIntervals)
		if err != nil {
			cclog.ComponentError("MetricRouter", "MetricCache initialization failed:", err.Error())
			return err
		}
		for _, agg := range r.config.IntervalAgg {
			r.cache.AddAggregation(agg.Name, agg.Function, agg.Condition, agg.Tags, agg.Meta)
		}
	}
	r.config.dropMetrics = make(map[string]bool)
	for _, mname := range r.config.DropMetrics {
		r.config.dropMetrics[mname] = true
	}
	return nil
}

// StartTimer starts a timer which updates timestamp periodically
func (r *metricRouter) StartTimer() {
	m := make(chan time.Time)
	r.ticker.AddChannel(m)
	r.timerdone = make(chan bool)

	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		for {
			select {
			case <-r.timerdone:
				close(r.timerdone)
				cclog.ComponentDebug("MetricRouter", "TIMER DONE")
				return
			case t := <-m:
				r.timestamp = t
			}
		}
	}()
	cclog.ComponentDebug("MetricRouter", "TIMER START")
}

func getParamMap(point lp.CCMetric) map[string]interface{} {
	params := make(map[string]interface{})
	params["metric"] = point
	params["name"] = point.Name()
	for key, value := range point.Tags() {
		params[key] = value
	}
	for key, value := range point.Meta() {
		params[key] = value
	}
	for key, value := range point.Fields() {
		params[key] = value
	}
	params["timestamp"] = point.Time()
	return params
}

// DoAddTags adds a tag when condition is fullfiled
func (r *metricRouter) DoAddTags(point lp.CCMetric) {
	for _, m := range r.config.AddTags {
		var conditionMatches bool = false

		if m.Condition == "*" {
			conditionMatches = true
		} else {
			var err error
			conditionMatches, err = agg.EvalBoolCondition(m.Condition, getParamMap(point))
			if err != nil {
				cclog.ComponentError("MetricRouter", err.Error())
				conditionMatches = false
			}
		}
		if conditionMatches {
			point.AddTag(m.Key, m.Value)
		}
	}
}

// DoDelTags removes a tag when condition is fullfiled
func (r *metricRouter) DoDelTags(point lp.CCMetric) {
	for _, m := range r.config.DelTags {
		var conditionMatches bool = false

		if m.Condition == "*" {
			conditionMatches = true
		} else {
			var err error
			conditionMatches, err = agg.EvalBoolCondition(m.Condition, getParamMap(point))
			if err != nil {
				cclog.ComponentError("MetricRouter", err.Error())
				conditionMatches = false
			}
		}
		if conditionMatches {
			point.RemoveTag(m.Key)
		}
	}
}

// Conditional test whether a metric should be dropped
func (r *metricRouter) dropMetric(point lp.CCMetric) bool {
	// Simple drop check
	if _, ok := r.config.dropMetrics[point.Name()]; ok {
		return true
	}
	// Checking the dropping conditions
	for _, m := range r.config.DropMetricsIf {
		conditionMatches, err := agg.EvalBoolCondition(m, getParamMap(point))
		if conditionMatches || err != nil {
			return true
		}
	}
	return false
}

// Start starts the metric router
func (r *metricRouter) Start() {
	// start timer if configured
	r.timestamp = time.Now()
	if r.config.IntervalStamp {
		r.StartTimer()
	}

	// Router manager is done
	done := func() {
		close(r.done)
		cclog.ComponentDebug("MetricRouter", "DONE")
	}

	// Forward takes a received metric, adds or deletes tags
	// and forwards it to the output channels
	forward := func(point lp.CCMetric) {
		cclog.ComponentDebug("MetricRouter", "FORWARD", point)
		r.DoAddTags(point)
		r.DoDelTags(point)
		if new, ok := r.config.RenameMetrics[point.Name()]; ok {
			point.SetName(new)
		}
		r.DoAddTags(point)
		r.DoDelTags(point)

		for _, o := range r.outputs {
			o <- point
		}
	}

	// Foward message received from collector channel
	coll_forward := func(p lp.CCMetric) {
		// receive from metric collector
		p.AddTag("hostname", r.hostname)
		if r.config.IntervalStamp {
			p.SetTime(r.timestamp)
		}
		if !r.dropMetric(p) {
			forward(p)
		}
		// even if the metric is dropped, it is stored in the cache for
		// aggregations
		if r.config.NumCacheIntervals > 0 {
			r.cache.Add(p)
		}
	}

	// Foward message received from receivers channel
	recv_forward := func(p lp.CCMetric) {
		// receive from receive manager
		if r.config.IntervalStamp {
			p.SetTime(r.timestamp)
		}
		if !r.dropMetric(p) {
			forward(p)
		}
	}

	// Foward message received from cache channel
	cache_forward := func(p lp.CCMetric) {
		// receive from metric collector
		if !r.dropMetric(p) {
			p.AddTag("hostname", r.hostname)
			forward(p)
		}
	}

	// Start Metric Cache
	if r.config.NumCacheIntervals > 0 {
		r.cache.Start()
	}

	r.wg.Add(1)
	go func() {
		defer r.wg.Done()

		for {
			select {
			case <-r.done:
				done()
				return

			case p := <-r.coll_input:
				coll_forward(p)
				for i := 0; len(r.coll_input) > 0 && i < r.maxForward; i++ {
					coll_forward(<-r.coll_input)
				}

			case p := <-r.recv_input:
				recv_forward(p)
				for i := 0; len(r.recv_input) > 0 && i < r.maxForward; i++ {
					recv_forward(<-r.recv_input)
				}

			case p := <-r.cache_input:
				cache_forward(p)
				for i := 0; len(r.cache_input) > 0 && i < r.maxForward; i++ {
					cache_forward(<-r.cache_input)
				}
			}
		}
	}()
	cclog.ComponentDebug("MetricRouter", "STARTED")
}

// AddCollectorInput adds a channel between metric collector and metric router
func (r *metricRouter) AddCollectorInput(input chan lp.CCMetric) {
	r.coll_input = input
}

// AddReceiverInput adds a channel between metric receiver and metric router
func (r *metricRouter) AddReceiverInput(input chan lp.CCMetric) {
	r.recv_input = input
}

// AddOutput adds a output channel to the metric router
func (r *metricRouter) AddOutput(output chan lp.CCMetric) {
	r.outputs = append(r.outputs, output)
}

// Close finishes / stops the metric router
func (r *metricRouter) Close() {
	cclog.ComponentDebug("MetricRouter", "CLOSE")
	r.done <- true
	// wait for close of channel r.done
	<-r.done
	if r.config.IntervalStamp {
		cclog.ComponentDebug("MetricRouter", "TIMER CLOSE")
		r.timerdone <- true
		// wait for close of channel r.timerdone
		<-r.timerdone
	}
	if r.config.NumCacheIntervals > 0 {
		r.cache.Close()
		r.cachewg.Wait()
	}
}

// New creates a new initialized metric router
func New(ticker mct.MultiChanTicker, wg *sync.WaitGroup, routerConfigFile string) (MetricRouter, error) {
	r := new(metricRouter)
	err := r.Init(ticker, wg, routerConfigFile)
	if err != nil {
		return nil, err
	}
	return r, err
}
