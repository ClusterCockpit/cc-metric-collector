// Copyright (C) NHR@FAU, University Erlangen-Nuremberg.
// All rights reserved. This file is part of cc-lib.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package metricRouter

import (
	"fmt"
	"sync"
	"time"

	cclog "github.com/ClusterCockpit/cc-lib/v2/ccLogger"
	lp "github.com/ClusterCockpit/cc-lib/v2/ccMessage"
	agg "github.com/ClusterCockpit/cc-metric-collector/internal/metricAggregator"
	mct "github.com/ClusterCockpit/cc-metric-collector/pkg/multiChanTicker"
)

// rawDataBuffer holds raw metrics collected during one interval
type rawDataBuffer struct {
	startTime time.Time
	endTime   time.Time
	metrics   []lp.CCMessage
}

// rawDataAggregator collects raw metrics and computes aggregates at each interval
type rawDataAggregator struct {
	output          chan lp.CCMessage
	ticker          mct.MultiChanTicker
	wg              *sync.WaitGroup
	done            chan bool
	tickChan        chan time.Time
	lock            sync.Mutex
	buffer          *rawDataBuffer
	aggEngine       agg.MetricAggregator
	lastAggTime     time.Time     // Time of last aggregation
	aggIntervalSecs int           // Aggregate every N seconds
}

// RawDataAggregator is the interface for the raw data aggregator
type RawDataAggregator interface {
	Init(output chan lp.CCMessage, ticker mct.MultiChanTicker, wg *sync.WaitGroup, intervalSeconds int) error
	Start()
	Add(metric lp.CCMessage)
	AddAggregation(name, function, condition string, tags, meta map[string]string) error
	DeleteAggregation(name string) error
	Close()
}

// Init initializes the raw data aggregator
func (a *rawDataAggregator) Init(output chan lp.CCMessage, ticker mct.MultiChanTicker, wg *sync.WaitGroup, intervalSeconds int) error {
	a.done = make(chan bool)
	a.wg = wg
	a.ticker = ticker
	a.output = output
	a.aggIntervalSecs = intervalSeconds
	if a.aggIntervalSecs < 1 {
		a.aggIntervalSecs = 10 // Default: aggregate every 10 seconds
	}
	a.lastAggTime = time.Now()

	// Initialize the buffer
	a.buffer = &rawDataBuffer{
		startTime: time.Now(),
		endTime:   time.Now(),
		metrics:   make([]lp.CCMessage, 0, 1000), // Pre-allocate with reasonable capacity
	}

	// Create a new aggregation engine
	var err error
	a.aggEngine, err = agg.NewAggregator(output)
	if err != nil {
		return fmt.Errorf("RawDataAggregator: failed to create aggregator: %w", err)
	}

	return nil
}

// Start starts the raw data aggregator goroutine
func (a *rawDataAggregator) Start() {
	a.tickChan = make(chan time.Time)
	a.ticker.AddChannel(a.tickChan)

	done := func() {
		cclog.ComponentDebug("RawDataAggregator", "DONE")
		close(a.done)
	}

	// rotateBuffer resets the buffer and returns the old one for processing
	rotateBuffer := func(timestamp time.Time) *rawDataBuffer {
		a.lock.Lock()
		defer a.lock.Unlock()

		old := a.buffer
		// Keep old.endTime as the time of the last metric added (set in Add())
		// This ensures aggregate timestamp >= all metric timestamps in the interval

		// Create new buffer starting from this tick
		a.buffer = &rawDataBuffer{
			startTime: timestamp,
			endTime:   timestamp,
			metrics:   make([]lp.CCMessage, 0, cap(old.metrics)),
		}

		return old
	}

	a.wg.Go(func() {
		for {
			select {
			case <-a.done:
				done()
				return
			case tick := <-a.tickChan:
				// Check if enough time has passed since last aggregation
				elapsed := tick.Sub(a.lastAggTime)
				if elapsed.Seconds() < float64(a.aggIntervalSecs) {
					continue
				}
				a.lastAggTime = tick

				// Get the old buffer and reset for new interval
				oldBuffer := rotateBuffer(tick)

				if len(oldBuffer.metrics) > 0 {
					// Make a copy of metrics slice to avoid race conditions
					// The aggEngine.Eval may take some time, and we don't want
					// to block Add() calls during that time
					metricsCopy := make([]lp.CCMessage, len(oldBuffer.metrics))
					copy(metricsCopy, oldBuffer.metrics)

				// Evaluate aggregations
				// Use endTime as the timestamp for aggregated metrics
				// (time of the last measurement in the interval)
				a.aggEngine.Eval(oldBuffer.endTime, oldBuffer.endTime, metricsCopy)
				} else {
					cclog.ComponentDebug("RawDataAggregator", "EMPTY INTERVAL")
				}
			}
		}
	})
	cclog.ComponentDebug("RawDataAggregator", "STARTED")
}

// Add adds a metric to the current buffer
// This method is thread-safe and can be called from multiple goroutines
func (a *rawDataAggregator) Add(metric lp.CCMessage) {
	// Skip nil metrics
	if metric == nil {
		return
	}

	a.lock.Lock()
	defer a.lock.Unlock()

	// Update end time to latest metric time
	metricTime := metric.Time()
	if metricTime.After(a.buffer.endTime) {
		a.buffer.endTime = metricTime
	}

	// Append metric to buffer
	a.buffer.metrics = append(a.buffer.metrics, metric)
}

// AddAggregation adds a new aggregation function configuration
func (a *rawDataAggregator) AddAggregation(name, function, condition string, tags, meta map[string]string) error {
	return a.aggEngine.AddAggregation(name, function, condition, tags, meta)
}

// DeleteAggregation removes an aggregation function configuration
func (a *rawDataAggregator) DeleteAggregation(name string) error {
	return a.aggEngine.DeleteAggregation(name)
}

// Close stops the raw data aggregator
func (a *rawDataAggregator) Close() {
	cclog.ComponentDebug("RawDataAggregator", "CLOSE")
	a.done <- true
	<-a.done
}

// NewRawDataAggregator creates a new initialized raw data aggregator
func NewRawDataAggregator(output chan lp.CCMessage, ticker mct.MultiChanTicker, wg *sync.WaitGroup, intervalSeconds int) (RawDataAggregator, error) {
	a := new(rawDataAggregator)
	err := a.Init(output, ticker, wg, intervalSeconds)
	if err != nil {
		return nil, err
	}
	return a, nil
}
