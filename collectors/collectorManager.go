package collectors

import (
	"encoding/json"
	"os"
	"sync"
	"time"

	cclog "github.com/ClusterCockpit/cc-metric-collector/internal/ccLogger"
	lp "github.com/ClusterCockpit/cc-metric-collector/internal/ccMetric"
	mct "github.com/ClusterCockpit/cc-metric-collector/internal/multiChanTicker"
)

// Map of all available metric collectors
var AvailableCollectors = map[string]MetricCollector{

	"likwid":           new(LikwidCollector),
	"loadavg":          new(LoadavgCollector),
	"memstat":          new(MemstatCollector),
	"netstat":          new(NetstatCollector),
	"ibstat":           new(InfinibandCollector),
	"ibstat_perfquery": new(InfinibandPerfQueryCollector),
	"lustrestat":       new(LustreCollector),
	"cpustat":          new(CpustatCollector),
	"topprocs":         new(TopProcsCollector),
	"nvidia":           new(NvidiaCollector),
	"customcmd":        new(CustomCmdCollector),
	"diskstat":         new(DiskstatCollector),
	"tempstat":         new(TempCollector),
	"ipmistat":         new(IpmiCollector),
	"gpfs":             new(GpfsCollector),
	"cpufreq":          new(CPUFreqCollector),
	"cpufreq_cpuinfo":  new(CPUFreqCpuInfoCollector),
	"nfsstat":          new(NfsCollector),
}

type collectorManager struct {
	collectors []MetricCollector
	output     chan lp.CCMetric // List of all output channels
	done       chan bool        // channel to finish / stop metric collector manager
	ticker     mct.MultiChanTicker
	duration   time.Duration
	wg         *sync.WaitGroup
	config     map[string]json.RawMessage
}

// Metric collector access functions
type CollectorManager interface {
	Init(ticker mct.MultiChanTicker, duration time.Duration, wg *sync.WaitGroup, collectConfigFile string) error
	AddOutput(output chan lp.CCMetric)
	Start()
	Close()
}

// Init initializes a new metric collector manager by setting up:
// * output channels
// * done channel
// * wait group synchronization (from variable wg)
// * ticker (from variable ticker)
// * configuration (read from config file in variable collectConfigFile)
// Initialization is done for all configured collectors
func (cm *collectorManager) Init(ticker mct.MultiChanTicker, duration time.Duration, wg *sync.WaitGroup, collectConfigFile string) error {
	cm.collectors = make([]MetricCollector, 0)
	cm.output = nil
	cm.done = make(chan bool)
	cm.wg = wg
	cm.ticker = ticker
	cm.duration = duration

	// Read collector config file
	configFile, err := os.Open(collectConfigFile)
	if err != nil {
		cclog.Error(err.Error())
		return err
	}
	defer configFile.Close()
	jsonParser := json.NewDecoder(configFile)
	err = jsonParser.Decode(&cm.config)
	if err != nil {
		cclog.Error(err.Error())
		return err
	}

	// Initialize configured collectors
	for k, cfg := range cm.config {
		if _, found := AvailableCollectors[k]; !found {
			cclog.ComponentError("CollectorManager", "SKIP unknown collector", k)
			continue
		}
		c := AvailableCollectors[k]

		err = c.Init(cfg)
		if err != nil {
			cclog.ComponentError("CollectorManager", "Collector", k, "initialization failed:", err.Error())
			continue
		}
		cclog.ComponentDebug("CollectorManager", "ADD COLLECTOR", c.Name())
		cm.collectors = append(cm.collectors, c)
	}
	return nil
}

// Start starts the metric collector manager
func (cm *collectorManager) Start() {
	cm.wg.Add(1)
	tick := make(chan time.Time)
	cm.ticker.AddChannel(tick)

	go func() {
		// Collector manager is done
		done := func() {
			// close all metric collectors
			for _, c := range cm.collectors {
				c.Close()
			}
			cm.wg.Done()
			cclog.ComponentDebug("CollectorManager", "DONE")
		}

		// Wait for done signal or timer event
		for {
			select {
			case <-cm.done:
				done()
				return
			case t := <-tick:
				for _, c := range cm.collectors {
					// Wait for done signal or execute the collector
					select {
					case <-cm.done:
						done()
						return
					default:
						// Read metrics from collector c
						cclog.ComponentDebug("CollectorManager", c.Name(), t)
						c.Read(cm.duration, cm.output)
					}
				}
			}
		}
	}()

	// Collector manager is started
	cclog.ComponentDebug("CollectorManager", "STARTED")
}

// AddOutput adds the output channel to the metric collector manager
func (cm *collectorManager) AddOutput(output chan lp.CCMetric) {
	cm.output = output
}

// Close finishes / stops the metric collector manager
func (cm *collectorManager) Close() {
	cclog.ComponentDebug("CollectorManager", "CLOSE")
	cm.done <- true
}

// New creates a new initialized metric collector manager
func New(ticker mct.MultiChanTicker, duration time.Duration, wg *sync.WaitGroup, collectConfigFile string) (CollectorManager, error) {
	cm := &collectorManager{}
	err := cm.Init(ticker, duration, wg, collectConfigFile)
	if err != nil {
		return nil, err
	}
	return cm, err
}
