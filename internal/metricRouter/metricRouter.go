package metricRouter

import (
	"encoding/json"
	"os"
	"strings"
	"sync"
	"time"

	cclog "github.com/ClusterCockpit/cc-metric-collector/pkg/ccLogger"

	agg "github.com/ClusterCockpit/cc-metric-collector/internal/metricAggregator"
	lp "github.com/ClusterCockpit/cc-metric-collector/pkg/ccMetric"
	mct "github.com/ClusterCockpit/cc-metric-collector/pkg/multiChanTicker"
	units "github.com/ClusterCockpit/cc-units"
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
	HostnameTagName   string                               `json:"hostname_tag"`        // Key name used when adding the hostname to a metric (default 'hostname')
	AddTags           []metricRouterTagConfig              `json:"add_tags"`            // List of tags that are added when the condition is met
	DelTags           []metricRouterTagConfig              `json:"delete_tags"`         // List of tags that are removed when the condition is met
	IntervalAgg       []agg.MetricAggregatorIntervalConfig `json:"interval_aggregates"` // List of aggregation function processed at the end of an interval
	DropMetrics       []string                             `json:"drop_metrics"`        // List of metric names to drop. For fine-grained dropping use drop_metrics_if
	DropMetricsIf     []string                             `json:"drop_metrics_if"`     // List of evaluatable terms to drop metrics
	RenameMetrics     map[string]string                    `json:"rename_metrics"`      // Map to rename metric name from key to value
	IntervalStamp     bool                                 `json:"interval_timestamp"`  // Update timestamp periodically by ticker each interval?
	NumCacheIntervals int                                  `json:"num_cache_intervals"` // Number of intervals of cached metrics for evaluation
	MaxForward        int                                  `json:"max_forward"`         // Number of maximal forwarded metrics at one select
	NormalizeUnits    bool                                 `json:"normalize_units"`     // Check unit meta flag and normalize it using cc-units
	ChangeUnitPrefix  map[string]string                    `json:"change_unit_prefix"`  // Add prefix that should be applied to the metrics
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
	r.config.MaxForward = ROUTER_MAX_FORWARD
	r.config.HostnameTagName = "hostname"

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
	r.maxForward = 1
	if r.config.MaxForward > r.maxForward {
		r.maxForward = r.config.MaxForward
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
	var conditionMatches bool
	for _, m := range r.config.AddTags {
		if m.Condition == "*" {
			// Condition is always matched
			conditionMatches = true
		} else {
			// Evaluate condition
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
	var conditionMatches bool
	for _, m := range r.config.DelTags {
		if m.Condition == "*" {
			// Condition is always matched
			conditionMatches = true
		} else {
			// Evaluate condition
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
	if conditionMatches, ok := r.config.dropMetrics[point.Name()]; ok {
		return conditionMatches
	}

	// Checking the dropping conditions
	for _, m := range r.config.DropMetricsIf {
		conditionMatches, err := agg.EvalBoolCondition(m, getParamMap(point))
		if err != nil {
			cclog.ComponentError("MetricRouter", err.Error())
			conditionMatches = false
		}
		if conditionMatches {
			return conditionMatches
		}
	}

	// No dropping condition met
	return false
}

func (r *metricRouter) prepareUnit(point lp.CCMetric) bool {
	if r.config.NormalizeUnits {
		if in_unit, ok := point.GetMeta("unit"); ok {
			u := units.NewUnit(in_unit)
			if u.Valid() {
				point.AddMeta("unit", u.Short())
			}
		}
	}
	if newP, ok := r.config.ChangeUnitPrefix[point.Name()]; ok {

		newPrefix := units.NewPrefix(newP)

		if in_unit, ok := point.GetMeta("unit"); ok && newPrefix != units.InvalidPrefix {
			u := units.NewUnit(in_unit)
			if u.Valid() {
				cclog.ComponentDebug("MetricRouter", "Change prefix to", newP, "for metric", point.Name())
				conv, out_unit := units.GetUnitPrefixFactor(u, newPrefix)
				if conv != nil && out_unit.Valid() {
					if val, ok := point.GetField("value"); ok {
						point.AddField("value", conv(val))
						point.AddMeta("unit", out_unit.Short())
					}
				}
			}

		}
	}

	return true
}

// Start starts the metric router
func (r *metricRouter) Start() {
	// start timer if configured
	r.timestamp = time.Now()
	timeChan := make(chan time.Time)
	if r.config.IntervalStamp {
		r.ticker.AddChannel(timeChan)
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
		name := point.Name()
		if new, ok := r.config.RenameMetrics[name]; ok {
			point.SetName(new)
			point.AddMeta("oldname", name)
			r.DoAddTags(point)
			r.DoDelTags(point)
		}

		r.prepareUnit(point)

		for _, o := range r.outputs {
			o <- point
		}
	}

	// Foward message received from collector channel
	coll_forward := func(p lp.CCMetric) {
		// receive from metric collector
		p.AddTag(r.config.HostnameTagName, r.hostname)
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

	// Forward message received from receivers channel
	recv_forward := func(p lp.CCMetric) {
		// receive from receive manager
		if r.config.IntervalStamp {
			p.SetTime(r.timestamp)
		}
		if !r.dropMetric(p) {
			forward(p)
		}
	}

	// Forward message received from cache channel
	cache_forward := func(p lp.CCMetric) {
		// receive from metric collector
		if !r.dropMetric(p) {
			p.AddTag(r.config.HostnameTagName, r.hostname)
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

			case timestamp := <-timeChan:
				r.timestamp = timestamp
				cclog.ComponentDebug("MetricRouter", "Update timestamp", r.timestamp.UnixNano())

			case p := <-r.coll_input:
				coll_forward(p)
				for i := 0; len(r.coll_input) > 0 && i < (r.maxForward-1); i++ {
					coll_forward(<-r.coll_input)
				}

			case p := <-r.recv_input:
				recv_forward(p)
				for i := 0; len(r.recv_input) > 0 && i < (r.maxForward-1); i++ {
					recv_forward(<-r.recv_input)
				}

			case p := <-r.cache_input:
				cache_forward(p)
				for i := 0; len(r.cache_input) > 0 && i < (r.maxForward-1); i++ {
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

	// stop metric cache
	if r.config.NumCacheIntervals > 0 {
		cclog.ComponentDebug("MetricRouter", "CACHE CLOSE")
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
