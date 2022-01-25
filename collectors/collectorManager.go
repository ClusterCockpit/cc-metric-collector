package collectors

import (
	"encoding/json"
	"os"
	"sync"
	"time"

	lp "github.com/ClusterCockpit/cc-metric-collector/internal/ccMetric"
	mct "github.com/ClusterCockpit/cc-metric-collector/internal/multiChanTicker"
	cclog "github.com/ClusterCockpit/cc-metric-collector/internal/ccLogger"
)

var AvailableCollectors = map[string]MetricCollector{

	"likwid":          &LikwidCollector{},
	"loadavg":         &LoadavgCollector{},
	"memstat":         &MemstatCollector{},
	"netstat":         &NetstatCollector{},
	"ibstat":          &InfinibandCollector{},
	"lustrestat":      &LustreCollector{},
	"cpustat":         &CpustatCollector{},
	"topprocs":        &TopProcsCollector{},
	"nvidia":          &NvidiaCollector{},
	"customcmd":       &CustomCmdCollector{},
	"diskstat":        &DiskstatCollector{},
	"tempstat":        &TempCollector{},
	"ipmistat":        &IpmiCollector{},
	"gpfs":            new(GpfsCollector),
	"cpufreq":         new(CPUFreqCollector),
	"cpufreq_cpuinfo": new(CPUFreqCpuInfoCollector),
	"nfsstat":         new(NfsCollector),
}

type collectorManager struct {
	collectors []MetricCollector
	output     chan lp.CCMetric
	done       chan bool
	ticker     mct.MultiChanTicker
	duration   time.Duration
	wg         *sync.WaitGroup
	config     map[string]json.RawMessage
}

type CollectorManager interface {
	Init(ticker mct.MultiChanTicker, duration time.Duration, wg *sync.WaitGroup, collectConfigFile string) error
	AddOutput(output chan lp.CCMetric)
	Start()
	Close()
}

func (cm *collectorManager) Init(ticker mct.MultiChanTicker, duration time.Duration, wg *sync.WaitGroup, collectConfigFile string) error {
	cm.collectors = make([]MetricCollector, 0)
	cm.output = nil
	cm.done = make(chan bool)
	cm.wg = wg
	cm.ticker = ticker
	cm.duration = duration
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

func (cm *collectorManager) Start() {
	cm.wg.Add(1)
	tick := make(chan time.Time)
	cm.ticker.AddChannel(tick)
	go func() {
		for {
		CollectorManagerLoop:
			select {
			case <-cm.done:
				for _, c := range cm.collectors {
					c.Close()
				}
				cm.wg.Done()
				cclog.ComponentDebug("CollectorManager", "DONE")
				break CollectorManagerLoop
			case t := <-tick:
				for _, c := range cm.collectors {
				CollectorManagerInputLoop:
					select {
					case <-cm.done:
						for _, c := range cm.collectors {
							c.Close()
						}
						cm.wg.Done()
						cclog.ComponentDebug("CollectorManager", "DONE")
						break CollectorManagerInputLoop
					default:
					    cclog.ComponentDebug("CollectorManager", c.Name(), t)
						c.Read(cm.duration, cm.output)
					}
				}
			}
		}
	}()
	cclog.ComponentDebug("CollectorManager", "STARTED")
}

func (cm *collectorManager) AddOutput(output chan lp.CCMetric) {
	cm.output = output
}

func (cm *collectorManager) Close() {
	cm.done <- true
	cclog.ComponentDebug("CollectorManager", "CLOSE")
}

func New(ticker mct.MultiChanTicker, duration time.Duration, wg *sync.WaitGroup, collectConfigFile string) (CollectorManager, error) {
	cm := &collectorManager{}
	err := cm.Init(ticker, duration, wg, collectConfigFile)
	if err != nil {
		return nil, err
	}
	return cm, err
}
