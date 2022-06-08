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

	"likwid":          new(LikwidCollector),
	"loadavg":         new(LoadavgCollector),
	"memstat":         new(MemstatCollector),
	"netstat":         new(NetstatCollector),
	"ibstat":          new(InfinibandCollector),
	"lustrestat":      new(LustreCollector),
	"cpustat":         new(CpustatCollector),
	"topprocs":        new(TopProcsCollector),
	"nvidia":          new(NvidiaCollector),
	"customcmd":       new(CustomCmdCollector),
	"iostat":          new(IOstatCollector),
	"diskstat":        new(DiskstatCollector),
	"tempstat":        new(TempCollector),
	"ipmistat":        new(IpmiCollector),
	"gpfs":            new(GpfsCollector),
	"cpufreq":         new(CPUFreqCollector),
	"cpufreq_cpuinfo": new(CPUFreqCpuInfoCollector),
	"nfs3stat":        new(Nfs3Collector),
	"nfs4stat":        new(Nfs4Collector),
	"numastats":       new(NUMAStatsCollector),
	"beegfs_meta":     new(BeegfsMetaCollector),
	"beegfs_storage":  new(BeegfsStorageCollector),
	"rocm_smi":        new(RocmSmiCollector),
}

// Metric collector manager data structure
type collectorManager struct {
	collectors   []MetricCollector          // List of metric collectors to read in parallel
	serial       []MetricCollector          // List of metric collectors to read serially
	output       chan lp.CCMetric           // Output channels
	done         chan bool                  // channel to finish / stop metric collector manager
	ticker       mct.MultiChanTicker        // periodically ticking once each interval
	duration     time.Duration              // duration (for metrics that measure over a given duration)
	wg           *sync.WaitGroup            // wait group for all goroutines in cc-metric-collector
	config       map[string]json.RawMessage // json encoded config for collector manager
	collector_wg sync.WaitGroup             // internally used wait group for the parallel reading of collector
	parallel_run bool                       // Flag whether the collectors are currently read in parallel
}

// Metric collector manager access functions
type CollectorManager interface {
	Init(ticker mct.MultiChanTicker, duration time.Duration, wg *sync.WaitGroup, collectConfigFile string) error
	AddOutput(output chan lp.CCMetric)
	Start()
	Close()
}

// Init initializes a new metric collector manager by setting up:
// * output channel
// * done channel
// * wait group synchronization for goroutines (from variable wg)
// * ticker (from variable ticker)
// * configuration (read from config file in variable collectConfigFile)
// Initialization is done for all configured collectors
func (cm *collectorManager) Init(ticker mct.MultiChanTicker, duration time.Duration, wg *sync.WaitGroup, collectConfigFile string) error {
	cm.collectors = make([]MetricCollector, 0)
	cm.serial = make([]MetricCollector, 0)
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
	for collectorName, collectorCfg := range cm.config {
		if _, found := AvailableCollectors[collectorName]; !found {
			cclog.ComponentError("CollectorManager", "SKIP unknown collector", collectorName)
			continue
		}
		collector := AvailableCollectors[collectorName]

		err = collector.Init(collectorCfg)
		if err != nil {
			cclog.ComponentError("CollectorManager", "Collector", collectorName, "initialization failed:", err.Error())
			continue
		}
		cclog.ComponentDebug("CollectorManager", "ADD COLLECTOR", collector.Name())
		if collector.Parallel() {
			cm.collectors = append(cm.collectors, collector)
		} else {
			cm.serial = append(cm.serial, collector)
		}
	}
	return nil
}

// Start starts the metric collector manager
func (cm *collectorManager) Start() {
	tick := make(chan time.Time)
	cm.ticker.AddChannel(tick)

	cm.wg.Add(1)
	go func() {
		defer cm.wg.Done()
		// Collector manager is done
		done := func() {
			// close all metric collectors
			if cm.parallel_run {
				cm.collector_wg.Wait()
				cm.parallel_run = false
			}
			for _, c := range cm.collectors {
				c.Close()
			}
			close(cm.done)
			cclog.ComponentDebug("CollectorManager", "DONE")
		}

		// Wait for done signal or timer event
		for {
			select {
			case <-cm.done:
				done()
				return
			case t := <-tick:
				cm.parallel_run = true
				for _, c := range cm.collectors {
					// Wait for done signal or execute the collector
					select {
					case <-cm.done:
						done()
						return
					default:
						// Read metrics from collector c via goroutine
						cclog.ComponentDebug("CollectorManager", c.Name(), t)
						cm.collector_wg.Add(1)
						go func(myc MetricCollector) {
							myc.Read(cm.duration, cm.output)
							cm.collector_wg.Done()
						}(c)
					}
				}
				cm.collector_wg.Wait()
				cm.parallel_run = false
				for _, c := range cm.serial {
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
	// wait for close of channel cm.done
	<-cm.done
}

// New creates a new initialized metric collector manager
func New(ticker mct.MultiChanTicker, duration time.Duration, wg *sync.WaitGroup, collectConfigFile string) (CollectorManager, error) {
	cm := new(collectorManager)
	err := cm.Init(ticker, duration, wg, collectConfigFile)
	if err != nil {
		return nil, err
	}
	return cm, err
}
