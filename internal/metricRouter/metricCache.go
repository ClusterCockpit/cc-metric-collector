package metricRouter

import (
	"sync"
	"time"

	cclog "github.com/ClusterCockpit/cc-metric-collector/pkg/ccLogger"

	agg "github.com/ClusterCockpit/cc-metric-collector/internal/metricAggregator"
	lp "github.com/ClusterCockpit/cc-metric-collector/pkg/ccMetric"
	mct "github.com/ClusterCockpit/cc-metric-collector/pkg/multiChanTicker"
)

type metricCachePeriod struct {
	startstamp  time.Time
	stopstamp   time.Time
	numMetrics  int
	sizeMetrics int
	metrics     []lp.CCMetric
}

// Metric cache data structure
type metricCache struct {
	numPeriods int
	curPeriod  int
	lock       sync.Mutex
	intervals  []*metricCachePeriod
	wg         *sync.WaitGroup
	ticker     mct.MultiChanTicker
	tickchan   chan time.Time
	done       chan bool
	output     chan lp.CCMetric
	aggEngine  agg.MetricAggregator
}

type MetricCache interface {
	Init(output chan lp.CCMetric, ticker mct.MultiChanTicker, wg *sync.WaitGroup, numPeriods int) error
	Start()
	Add(metric lp.CCMetric)
	GetPeriod(index int) (time.Time, time.Time, []lp.CCMetric)
	AddAggregation(name, function, condition string, tags, meta map[string]string) error
	DeleteAggregation(name string) error
	Close()
}

func (c *metricCache) Init(output chan lp.CCMetric, ticker mct.MultiChanTicker, wg *sync.WaitGroup, numPeriods int) error {
	var err error = nil
	c.done = make(chan bool)
	c.wg = wg
	c.ticker = ticker
	c.numPeriods = numPeriods
	c.output = output
	c.intervals = make([]*metricCachePeriod, 0)
	for i := 0; i < c.numPeriods+1; i++ {
		p := new(metricCachePeriod)
		p.numMetrics = 0
		p.sizeMetrics = 0
		p.metrics = make([]lp.CCMetric, 0)
		c.intervals = append(c.intervals, p)
	}

	// Create a new aggregation engine. No separate goroutine at the moment
	// The code is executed by the MetricCache goroutine
	c.aggEngine, err = agg.NewAggregator(c.output)
	if err != nil {
		cclog.ComponentError("MetricCache", "Cannot create aggregator")
		return err
	}

	return nil
}

// Start starts the metric cache
func (c *metricCache) Start() {

	c.tickchan = make(chan time.Time)
	c.ticker.AddChannel(c.tickchan)
	// Router cache is done
	done := func() {
		cclog.ComponentDebug("MetricCache", "DONE")
		close(c.done)
	}

	// Rotate cache interval
	rotate := func(timestamp time.Time) int {
		oldPeriod := c.curPeriod
		c.curPeriod = oldPeriod + 1
		if c.curPeriod >= c.numPeriods {
			c.curPeriod = 0
		}
		c.intervals[oldPeriod].numMetrics = 0
		c.intervals[oldPeriod].stopstamp = timestamp
		c.intervals[c.curPeriod].startstamp = timestamp
		c.intervals[c.curPeriod].stopstamp = timestamp
		return oldPeriod
	}

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		for {
			select {
			case <-c.done:
				done()
				return
			case tick := <-c.tickchan:
				c.lock.Lock()
				old := rotate(tick)
				// Get the last period and evaluate aggregation metrics
				starttime, endtime, metrics := c.GetPeriod(old)
				c.lock.Unlock()
				if len(metrics) > 0 {
					c.aggEngine.Eval(starttime, endtime, metrics)
				} else {
					// This message is also printed in the first interval after startup
					cclog.ComponentDebug("MetricCache", "EMPTY INTERVAL?")
				}
			}
		}
	}()
	cclog.ComponentDebug("MetricCache", "START")
}

// Add a metric to the cache. The interval is defined by the global timer (rotate() in Start())
// The intervals list is used as round-robin buffer and the metric list grows dynamically and
// to avoid reallocations
func (c *metricCache) Add(metric lp.CCMetric) {
	if c.curPeriod >= 0 && c.curPeriod < c.numPeriods {
		c.lock.Lock()
		p := c.intervals[c.curPeriod]
		if p.numMetrics < p.sizeMetrics {
			p.metrics[p.numMetrics] = metric
			p.numMetrics = p.numMetrics + 1
			p.stopstamp = metric.Time()
		} else {
			p.metrics = append(p.metrics, metric)
			p.numMetrics = p.numMetrics + 1
			p.sizeMetrics = p.sizeMetrics + 1
			p.stopstamp = metric.Time()
		}
		c.lock.Unlock()
	}
}

func (c *metricCache) AddAggregation(name, function, condition string, tags, meta map[string]string) error {
	return c.aggEngine.AddAggregation(name, function, condition, tags, meta)
}

func (c *metricCache) DeleteAggregation(name string) error {
	return c.aggEngine.DeleteAggregation(name)
}

// Get all metrics of a interval. The index is the difference to the current interval, so index=0
// is the current one, index=1 the last interval and so on. Returns and empty array if a wrong index
// is given (negative index, index larger than configured number of total intervals, ...)
func (c *metricCache) GetPeriod(index int) (time.Time, time.Time, []lp.CCMetric) {
	var start time.Time = time.Now()
	var stop time.Time = time.Now()
	var metrics []lp.CCMetric
	if index >= 0 && index < c.numPeriods {
		pindex := c.curPeriod - index
		if pindex < 0 {
			pindex = c.numPeriods - pindex
		}
		if pindex >= 0 && pindex < c.numPeriods {
			start = c.intervals[pindex].startstamp
			stop = c.intervals[pindex].stopstamp
			metrics = c.intervals[pindex].metrics
			//return c.intervals[pindex].startstamp, c.intervals[pindex].stopstamp, c.intervals[pindex].metrics
		} else {
			metrics = make([]lp.CCMetric, 0)
		}
	} else {
		metrics = make([]lp.CCMetric, 0)
	}
	return start, stop, metrics
}

// Close finishes / stops the metric cache
func (c *metricCache) Close() {
	cclog.ComponentDebug("MetricCache", "CLOSE")
	c.done <- true
}

func NewCache(output chan lp.CCMetric, ticker mct.MultiChanTicker, wg *sync.WaitGroup, numPeriods int) (MetricCache, error) {
	c := new(metricCache)
	err := c.Init(output, ticker, wg, numPeriods)
	if err != nil {
		return nil, err
	}
	return c, err
}
