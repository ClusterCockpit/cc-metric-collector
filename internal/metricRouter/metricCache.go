// Copyright (C) NHR@FAU, University Erlangen-Nuremberg.
// All rights reserved. This file is part of cc-lib.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.
// additional authors:
// Holger Obermaier (NHR@KIT)

package metricRouter

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	cclog "github.com/ClusterCockpit/cc-lib/v2/ccLogger"

	lp "github.com/ClusterCockpit/cc-lib/v2/ccMessage"
	agg "github.com/ClusterCockpit/cc-metric-collector/internal/metricAggregator"
	mct "github.com/ClusterCockpit/cc-metric-collector/pkg/multiChanTicker"
)

type ccCache struct {
	periodIdx   int
	maxPeriods  int
	periods     [][]lp.CCMessage
	periodTimes []struct {
		starttime time.Time
		endtime   time.Time
	}
}

type CCCache interface {
	Init(numPeriods int) error
	Add(msg lp.CCMessage) error
	GetPeriod(offset int) (time.Time, time.Time, []lp.CCMessage)
	GetAll() []lp.CCMessage
	NewPeriod()
}

func (c *ccCache) Init(numPeriods int) error {
	c.maxPeriods = numPeriods
	c.periodIdx = 0
	c.periods = make([][]lp.CCMessage, c.maxPeriods)
	c.periodTimes = make([]struct {
		starttime time.Time
		endtime   time.Time
	}, c.maxPeriods)
	return nil
}

func (c *ccCache) NewPeriod() {
	c.periodTimes[c.periodIdx].endtime = time.Now()
	c.periodIdx = (c.periodIdx + 1) % c.maxPeriods
	fmt.Printf("New period index %d\n", c.periodIdx)
	c.periods[c.periodIdx] = c.periods[c.periodIdx][:0]
	c.periodTimes[c.periodIdx].starttime = time.Now()
	c.periodTimes[c.periodIdx].endtime = c.periodTimes[c.periodIdx].starttime
}

func (c *ccCache) Add(msg lp.CCMessage) error {
	c.periods[c.periodIdx] = append(c.periods[c.periodIdx], msg)
	c.periodTimes[c.periodIdx].endtime = msg.Time()
	return nil
}

func (c *ccCache) GetPeriod(offset int) (time.Time, time.Time, []lp.CCMessage) {
	if offset > c.maxPeriods {
		offset = offset % c.maxPeriods
	}
	poff := int(math.Abs(float64(c.periodIdx - offset)))
	out := make([]lp.CCMessage, 0, len(c.periods[poff%c.maxPeriods]))
	out = append(out, c.periods[poff%c.maxPeriods]...)

	return c.periodTimes[poff%c.maxPeriods].starttime, c.periodTimes[poff%c.maxPeriods].endtime, out
}

func (c *ccCache) GetAll() []lp.CCMessage {
	out := make([]lp.CCMessage, 0)
	for _, data := range c.periods {
		out = append(out, data...)
	}
	return out
}

// Metric cache data structure
type metricCache struct {
	cache      CCCache
	wg         *sync.WaitGroup
	ticker     mct.MultiChanTicker
	tickchan   chan time.Time
	done       chan bool
	output     chan lp.CCMessage
	aggEngine  agg.MetricAggregator
	numPeriods int
	started    bool
}

type MetricCache interface {
	Init(output chan lp.CCMessage, ticker mct.MultiChanTicker, wg *sync.WaitGroup, interval time.Duration, numPeriods int) error
	Start()
	Add(metric lp.CCMessage)
	AddAggregation(name, function, condition string, tags, meta map[string]string) error
	DeleteAggregation(name string) error
	Close()
}

func (c *metricCache) Init(output chan lp.CCMessage, ticker mct.MultiChanTicker, wg *sync.WaitGroup, interval time.Duration, numPeriods int) error {
	var err error
	c.done = make(chan bool)
	c.wg = wg
	c.ticker = ticker
	c.numPeriods = numPeriods
	c.started = false
	c.cache = new(ccCache)
	c.output = output

	err = c.cache.Init(numPeriods)
	if err != nil {
		return fmt.Errorf("MetricCache: failed to create cache: %w", err)
	}

	c.aggEngine, err = agg.NewAggregatorExpr(c.output)
	if err != nil {
		return fmt.Errorf("MetricCache: failed to create aggregator: %w", err)
	}

	return nil
}

// Start starts the metric cache
func (c *metricCache) Start() {
	c.tickchan = make(chan time.Time)
	c.ticker.AddChannel(c.tickchan)

	c.wg.Add(1)
	go func() {
		for {
			select {
			case <-c.done:
				c.wg.Done()
				close(c.done)
				cclog.ComponentDebug("MetricCache", "DONE")

				return
			case tick := <-c.tickchan:
				cclog.ComponentDebug("MetricCache", "Tick", tick)
				allmetrics := c.cache.GetAll()
				c.cache.NewPeriod()
				mintime := tick
				maxtime := mintime.AddDate(-1, 0, 0)

				for _, metric := range allmetrics {
					if metric.Time().Before(mintime) {
						mintime = metric.Time()
					}
					if metric.Time().After(maxtime) {
						maxtime = metric.Time()
					}
				}
				if len(allmetrics) > 0 {
					cclog.ComponentDebugf("MetricCache", "Evaluate %d metrics from %v to %v", len(allmetrics), mintime.UnixNano(), maxtime.UnixNano())
					c.wg.Go(func() {
						c.aggEngine.Eval(mintime, maxtime, allmetrics)
					})
				} else {
					// This message is also printed in the first interval after startup
					cclog.ComponentDebug("MetricCache", "EMPTY INTERVAL?")
				}

			}
		}
	}()

	cclog.ComponentDebug("MetricCache", "STARTED")
}

// Add a metric to the cache. The interval is defined by the global timer (rotate() in Start())
// The intervals list is used as round-robin buffer and the metric list grows dynamically and
// to avoid reallocations
func (c *metricCache) Add(metric lp.CCMessage) {
	err := c.cache.Add(metric)
	if err != nil {
		s := metric.ToLineProtocol(nil)
		s = strings.TrimSpace(s)
		cclog.ComponentErrorf("MetricCache", "Failed to add metric %s", s)
	}
}

func (c *metricCache) AddAggregation(name, function, condition string, tags, meta map[string]string) error {
	return c.aggEngine.AddAggregation(name, function, condition, tags, meta)
}

func (c *metricCache) DeleteAggregation(name string) error {
	return c.aggEngine.DeleteAggregation(name)
}

// Close finishes / stops the metric cache
func (c *metricCache) Close() {
	cclog.ComponentDebug("MetricCache", "CLOSE")
	if c.started {
		c.done <- true
		c.wg.Wait()

	}
	cclog.ComponentDebug("MetricCache", "CLOSED")
}

func NewCache(output chan lp.CCMessage, ticker mct.MultiChanTicker, wg *sync.WaitGroup, numPeriods int) (MetricCache, error) {
	c := new(metricCache)
	err := c.Init(output, ticker, wg, ticker.GetDuration(), numPeriods)
	if err != nil {
		return nil, err
	}
	return c, err
}
