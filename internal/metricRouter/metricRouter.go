// Copyright (C) NHR@FAU, University Erlangen-Nuremberg.
// All rights reserved. This file is part of cc-lib.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.
// additional authors:
// Holger Obermaier (NHR@KIT)

package metricRouter

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"strings"
	"sync"
	"time"

	cclog "github.com/ClusterCockpit/cc-lib/v2/ccLogger"

	lp "github.com/ClusterCockpit/cc-lib/v2/ccMessage"
	mp "github.com/ClusterCockpit/cc-lib/v2/messageProcessor"
	agg "github.com/ClusterCockpit/cc-metric-collector/internal/metricAggregator"
	mct "github.com/ClusterCockpit/cc-metric-collector/pkg/multiChanTicker"
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
	// dropMetrics       map[string]bool                      // Internal map for O(1) lookup
	MessageProcessor json.RawMessage `json:"process_messages,omitempty"`
}

// Metric router data structure
type metricRouter struct {
	hostname    string              // Hostname used in tags
	coll_input  chan lp.CCMessage   // Input channel from CollectorManager
	recv_input  chan lp.CCMessage   // Input channel from ReceiveManager
	cache_input chan lp.CCMessage   // Input channel from MetricCache
	outputs     []chan lp.CCMessage // List of all output channels
	done        chan bool           // channel to finish / stop metric router
	wg          *sync.WaitGroup     // wait group for all goroutines in cc-metric-collector
	timestamp   time.Time           // timestamp periodically updated by ticker each interval
	ticker      mct.MultiChanTicker // periodically ticking once each interval
	config      metricRouterConfig  // json encoded config for metric router
	cache       MetricCache         // pointer to MetricCache
	cachewg     sync.WaitGroup      // wait group for MetricCache
	maxForward  int                 // number of metrics to forward maximally in one iteration
	mp          mp.MessageProcessor
}

// MetricRouter access functions
type MetricRouter interface {
	Init(ticker mct.MultiChanTicker, wg *sync.WaitGroup, routerConfig json.RawMessage) error
	AddCollectorInput(input chan lp.CCMessage)
	AddReceiverInput(input chan lp.CCMessage)
	AddOutput(output chan lp.CCMessage)
	Start()
	Close()
}

// Init initializes a metric router by setting up:
// * input and output channels
// * done channel
// * wait group synchronization (from variable wg)
// * ticker (from variable ticker)
// * configuration (read from config file in variable routerConfigFile)
func (r *metricRouter) Init(ticker mct.MultiChanTicker, wg *sync.WaitGroup, routerConfig json.RawMessage) error {
	r.outputs = make([]chan lp.CCMessage, 0)
	r.done = make(chan bool)
	r.cache_input = make(chan lp.CCMessage)
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

	err = json.Unmarshal(routerConfig, &r.config)
	if err != nil {
		cclog.ComponentError("MetricRouter", err.Error())
		return err
	}
	r.maxForward = max(1, r.config.MaxForward)

	if r.config.NumCacheIntervals > 0 {
		r.cache, err = NewCache(r.cache_input, r.ticker, &r.cachewg, r.config.NumCacheIntervals)
		if err != nil {
			cclog.ComponentError("MetricRouter", "MetricCache initialization failed:", err.Error())
			return err
		}
		for _, agg := range r.config.IntervalAgg {
			err = r.cache.AddAggregation(agg.Name, agg.Function, agg.Condition, agg.Tags, agg.Meta)
			if err != nil {
				return fmt.Errorf("MetricCache AddAggregation() failed: %w", err)
			}
		}
	}
	p, err := mp.NewMessageProcessor()
	if err != nil {
		return fmt.Errorf("MessageProcessor NewMessageProcessor() failed: %w", err)
	}
	r.mp = p

	if len(r.config.MessageProcessor) > 0 {
		err = r.mp.FromConfigJSON(r.config.MessageProcessor)
		if err != nil {
			return fmt.Errorf("MessageProcessor FromConfigJSON() failed: %w", err)
		}
	}
	for _, mname := range r.config.DropMetrics {
		err = r.mp.AddDropMessagesByName(mname)
		if err != nil {
			return fmt.Errorf("MessageProcessor AddDropMessagesByName() failed: %w", err)
		}
	}
	for _, cond := range r.config.DropMetricsIf {
		err = r.mp.AddDropMessagesByCondition(cond)
		if err != nil {
			return fmt.Errorf("MessageProcessor AddDropMessagesByCondition() failed: %w", err)
		}
	}
	for _, data := range r.config.AddTags {
		cond := data.Condition
		if cond == "*" {
			cond = "true"
		}
		err = r.mp.AddAddTagsByCondition(cond, data.Key, data.Value)
		if err != nil {
			return fmt.Errorf("MessageProcessor AddAddTagsByCondition() failed: %w", err)
		}
	}
	for _, data := range r.config.DelTags {
		cond := data.Condition
		if cond == "*" {
			cond = "true"
		}
		err = r.mp.AddDeleteTagsByCondition(cond, data.Key, data.Value)
		if err != nil {
			return fmt.Errorf("MessageProcessor AddDeleteTagsByCondition() failed: %w", err)
		}
	}
	for oldname, newname := range r.config.RenameMetrics {
		err = r.mp.AddRenameMetricByName(oldname, newname)
		if err != nil {
			return fmt.Errorf("MessageProcessor AddRenameMetricByName() failed: %w", err)
		}
	}
	for metricName, prefix := range r.config.ChangeUnitPrefix {
		err = r.mp.AddChangeUnitPrefix(fmt.Sprintf("name == '%s'", metricName), prefix)
		if err != nil {
			return fmt.Errorf("MessageProcessor AddChangeUnitPrefix() failed: %w", err)
		}
	}
	r.mp.SetNormalizeUnits(r.config.NormalizeUnits)

	err = r.mp.AddAddTagsByCondition("true", r.config.HostnameTagName, r.hostname)
	if err != nil {
		return fmt.Errorf("MessageProcessor AddAddTagsByCondition() failed: %w", err)
	}

	// r.config.dropMetrics = make(map[string]bool)
	// for _, mname := range r.config.DropMetrics {
	// 	r.config.dropMetrics[mname] = true
	// }
	return nil
}

func getParamMap(point lp.CCMessage) map[string]any {
	params := make(map[string]any)
	params["metric"] = point
	params["name"] = point.Name()
	for key, value := range point.Tags() {
		params[key] = value
	}
	for key, value := range point.Meta() {
		params[key] = value
	}
	maps.Copy(params, point.Fields())
	params["timestamp"] = point.Time()
	return params
}

// DoAddTags adds a tag when condition is fullfiled
func (r *metricRouter) DoAddTags(point lp.CCMessage) {
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
// func (r *metricRouter) DoDelTags(point lp.CCMessage) {
// 	var conditionMatches bool
// 	for _, m := range r.config.DelTags {
// 		if m.Condition == "*" {
// 			// Condition is always matched
// 			conditionMatches = true
// 		} else {
// 			// Evaluate condition
// 			var err error
// 			conditionMatches, err = agg.EvalBoolCondition(m.Condition, getParamMap(point))
// 			if err != nil {
// 				cclog.ComponentError("MetricRouter", err.Error())
// 				conditionMatches = false
// 			}
// 		}
// 		if conditionMatches {
// 			point.RemoveTag(m.Key)
// 		}
// 	}
// }

// Conditional test whether a metric should be dropped
// func (r *metricRouter) dropMetric(point lp.CCMessage) bool {
// 	// Simple drop check
// 	if conditionMatches, ok := r.config.dropMetrics[point.Name()]; ok {
// 		return conditionMatches
// 	}

// 	// Checking the dropping conditions
// 	for _, m := range r.config.DropMetricsIf {
// 		conditionMatches, err := agg.EvalBoolCondition(m, getParamMap(point))
// 		if err != nil {
// 			cclog.ComponentError("MetricRouter", err.Error())
// 			conditionMatches = false
// 		}
// 		if conditionMatches {
// 			return conditionMatches
// 		}
// 	}

// 	// No dropping condition met
// 	return false
// }

// func (r *metricRouter) prepareUnit(point lp.CCMessage) bool {
// 	if r.config.NormalizeUnits {
// 		if in_unit, ok := point.GetMeta("unit"); ok {
// 			u := units.NewUnit(in_unit)
// 			if u.Valid() {
// 				point.AddMeta("unit", u.Short())
// 			}
// 		}
// 	}
// 	if newP, ok := r.config.ChangeUnitPrefix[point.Name()]; ok {

// 		newPrefix := units.NewPrefix(newP)

// 		if in_unit, ok := point.GetMeta("unit"); ok && newPrefix != units.InvalidPrefix {
// 			u := units.NewUnit(in_unit)
// 			if u.Valid() {
// 				cclog.ComponentDebug("MetricRouter", "Change prefix to", newP, "for metric", point.Name())
// 				conv, out_unit := units.GetUnitPrefixFactor(u, newPrefix)
// 				if conv != nil && out_unit.Valid() {
// 					if val, ok := point.GetField("value"); ok {
// 						point.AddField("value", conv(val))
// 						point.AddMeta("unit", out_unit.Short())
// 					}
// 				}
// 			}

// 		}
// 	}

// 	return true
// }

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
	// forward := func(point lp.CCMessage) {
	// 	cclog.ComponentDebug("MetricRouter", "FORWARD", point)
	// 	r.DoAddTags(point)
	// 	r.DoDelTags(point)
	// 	name := point.Name()
	// 	if new, ok := r.config.RenameMetrics[name]; ok {
	// 		point.SetName(new)
	// 		point.AddMeta("oldname", name)
	// 		r.DoAddTags(point)
	// 		r.DoDelTags(point)
	// 	}

	// 	r.prepareUnit(point)

	// 	for _, o := range r.outputs {
	// 		o <- point
	// 	}
	// }

	// Foward message received from collector channel
	coll_forward := func(p lp.CCMessage) {
		// receive from metric collector
		//p.AddTag(r.config.HostnameTagName, r.hostname)
		if r.config.IntervalStamp {
			p.SetTime(r.timestamp)
		}
		m, err := r.mp.ProcessMessage(p)
		if err == nil && m != nil {
			for _, o := range r.outputs {
				o <- m
			}
		}
		// if !r.dropMetric(p) {
		// 	for _, o := range r.outputs {
		// 		o <- point
		// 	}
		// }
		// even if the metric is dropped, it is stored in the cache for
		// aggregations
		if r.config.NumCacheIntervals > 0 {
			r.cache.Add(m)
		}
	}

	// Forward message received from receivers channel
	recv_forward := func(p lp.CCMessage) {
		// receive from receive manager
		if r.config.IntervalStamp {
			p.SetTime(r.timestamp)
		}
		m, err := r.mp.ProcessMessage(p)
		if err == nil && m != nil {
			for _, o := range r.outputs {
				o <- m
			}
		}
		// if !r.dropMetric(p) {
		// 	forward(p)
		// }
	}

	// Forward message received from cache channel
	cache_forward := func(p lp.CCMessage) {
		// receive from metric collector
		m, err := r.mp.ProcessMessage(p)
		if err == nil && m != nil {
			for _, o := range r.outputs {
				o <- m
			}
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
func (r *metricRouter) AddCollectorInput(input chan lp.CCMessage) {
	r.coll_input = input
}

// AddReceiverInput adds a channel between metric receiver and metric router
func (r *metricRouter) AddReceiverInput(input chan lp.CCMessage) {
	r.recv_input = input
}

// AddOutput adds a output channel to the metric router
func (r *metricRouter) AddOutput(output chan lp.CCMessage) {
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
func New(ticker mct.MultiChanTicker, wg *sync.WaitGroup, routerConfig json.RawMessage) (MetricRouter, error) {
	r := new(metricRouter)
	err := r.Init(ticker, wg, routerConfig)
	if err != nil {
		return nil, err
	}
	return r, err
}
