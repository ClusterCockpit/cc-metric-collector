package metricRouter

import (
	"encoding/json"
	"os"
	"strings"
	"sync"
	"time"

	cclog "github.com/ClusterCockpit/cc-metric-collector/internal/ccLogger"

	lp "github.com/ClusterCockpit/cc-metric-collector/internal/ccMetric"
	mct "github.com/ClusterCockpit/cc-metric-collector/internal/multiChanTicker"
	"gopkg.in/Knetic/govaluate.v2"
)

// Metric router tag configuration
type metricRouterTagConfig struct {
	Key       string `json:"key"`   // Tag name
	Value     string `json:"value"` // Tag value
	Condition string `json:"if"`    // Condition for adding or removing corresponding tag
}

// Metric router configuration
type metricRouterConfig struct {
	AddTags       []metricRouterTagConfig `json:"add_tags"`           // List of tags that are added when the condition is met
	DelTags       []metricRouterTagConfig `json:"delete_tags"`        // List of tags that are removed when the condition is met
	IntervalStamp bool                    `json:"interval_timestamp"` // Update timestamp periodically by ticker each interval?
}

// Metric router data structure
type metricRouter struct {
	hostname   string              // Hostname used in tags
	coll_input chan lp.CCMetric    // Input channel from CollectorManager
	recv_input chan lp.CCMetric    // Input channel from ReceiveManager
	outputs    []chan lp.CCMetric  // List of all output channels
	done       chan bool           // channel to finish / stop metric router
	wg         *sync.WaitGroup     // wait group for all goroutines in cc-metric-collector
	timestamp  time.Time           // timestamp periodically updated by ticker each interval
	timerdone  chan bool           // channel to finish / stop timestamp updater
	ticker     mct.MultiChanTicker // periodically ticking once each interval
	config     metricRouterConfig  // json encoded config for metric router
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
	r.wg = wg
	r.ticker = ticker

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
				cclog.ComponentDebug("MetricRouter", "TIMER DONE")
				return
			case t := <-m:
				r.timestamp = t
			}
		}
	}()
	cclog.ComponentDebug("MetricRouter", "TIMER START")
}

// EvalCondition evaluates condition cond for metric data from point
func (r *metricRouter) EvalCondition(cond string, point lp.CCMetric) (bool, error) {
	expression, err := govaluate.NewEvaluableExpression(cond)
	if err != nil {
		cclog.ComponentDebug("MetricRouter", cond, " = ", err.Error())
		return false, err
	}

	// Add metric name, tags, meta data, fields and timestamp to the parameter list
	params := make(map[string]interface{})
	params["name"] = point.Name()
	for _, t := range point.TagList() {
		params[t.Key] = t.Value
	}
	for _, m := range point.MetaList() {
		params[m.Key] = m.Value
	}
	for _, f := range point.FieldList() {
		params[f.Key] = f.Value
	}
	params["timestamp"] = point.Time()

	// evaluate condition
	result, err := expression.Evaluate(params)
	if err != nil {
		cclog.ComponentDebug("MetricRouter", cond, " = ", err.Error())
		return false, err
	}
	return bool(result.(bool)), err
}

// DoAddTags adds a tag when condition is fullfiled
func (r *metricRouter) DoAddTags(point lp.CCMetric) {
	for _, m := range r.config.AddTags {
		var conditionMatches bool

		if m.Condition == "*" {
			conditionMatches = true
		} else {
			var err error
			conditionMatches, err = r.EvalCondition(m.Condition, point)
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
		var conditionMatches bool

		if m.Condition == "*" {
			conditionMatches = true
		} else {
			var err error
			conditionMatches, err = r.EvalCondition(m.Condition, point)
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

// Start starts the metric router
func (r *metricRouter) Start() {

	// start timer if configured
	r.timestamp = time.Now()
	if r.config.IntervalStamp {
		r.StartTimer()
	}

	// Router manager is done
	done := func() {
		cclog.ComponentDebug("MetricRouter", "DONE")
	}

	// Forward takes a received metric, adds or deletes tags
	// and forwards it to the output channels
	forward := func(point lp.CCMetric) {
		cclog.ComponentDebug("MetricRouter", "FORWARD", point)
		r.DoAddTags(point)
		r.DoDelTags(point)
		for _, o := range r.outputs {
			o <- point
		}
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
				// receive from metric collector
				p.AddTag("hostname", r.hostname)
				if r.config.IntervalStamp {
					p.SetTime(r.timestamp)
				}
				forward(p)

			case p := <-r.recv_input:
				// receive from receive manager
				if r.config.IntervalStamp {
					p.SetTime(r.timestamp)
				}
				forward(p)
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
	if r.config.IntervalStamp {
		cclog.ComponentDebug("MetricRouter", "TIMER CLOSE")
		r.timerdone <- true
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
