package collectors

import (
    lp "github.com/ClusterCockpit/cc-metric-collector/internal/ccMetric"
    "sync"
    "time"
    "log"
    "os"
    "encoding/json"
)


var AvailableCollectors = map[string]MetricCollector{
	"likwid":     &LikwidCollector{},
	"loadavg":    &LoadavgCollector{},
	"memstat":    &MemstatCollector{},
	"netstat":    &NetstatCollector{},
	"ibstat":     &InfinibandCollector{},
	"lustrestat": &LustreCollector{},
	"cpustat":    &CpustatCollector{},
	"topprocs":   &TopProcsCollector{},
	"nvidia":     &NvidiaCollector{},
	"customcmd":  &CustomCmdCollector{},
	"diskstat":   &DiskstatCollector{},
	"tempstat":   &TempCollector{},
	"ipmistat":   &IpmiCollector{},
}


type collectorManager struct {
	collectors  []MetricCollector
	output      chan lp.CCMetric
	done        chan bool
	interval    time.Duration
	duration    time.Duration
	wg          *sync.WaitGroup
	config      map[string]json.RawMessage
}

type CollectorManager interface {
	Init(interval time.Duration, duration time.Duration, wg *sync.WaitGroup, collectConfigFile string) error
	AddOutput(output chan lp.CCMetric)
	Start()
	Close()
}


func (cm *collectorManager) Init(interval time.Duration, duration time.Duration, wg *sync.WaitGroup, collectConfigFile string) error {
    cm.collectors = make([]MetricCollector, 0)
    cm.output = nil
    cm.done = make(chan bool)
    cm.wg = wg
    cm.interval = interval
    cm.duration = duration
    configFile, err := os.Open(collectConfigFile)
    if err != nil {
        log.Print(err.Error())
        return err
    }
    defer configFile.Close()
    jsonParser := json.NewDecoder(configFile)
	err = jsonParser.Decode(&cm.config)
	if err != nil {
	    log.Print(err.Error())
        return err
    }
    for k, cfg := range cm.config {
        log.Print(k, " ", cfg)
        if _, found := AvailableCollectors[k]; !found {
            log.Print("[CollectorManager] SKIP unknown collector ", k)
            continue
        }
        c := AvailableCollectors[k]
        
        err = c.Init(cfg)
        if err != nil {
            log.Print("[CollectorManager] Collector ", k, "initialization failed: ", err.Error())
            continue
        }
        cm.collectors = append(cm.collectors, c)
    }
    return nil
}

func (cm *collectorManager) Start() {
    cm.wg.Add(1)
    ticker := time.NewTicker(cm.interval)
    go func() {
        for {
CollectorManagerLoop:
            select {
            case <- cm.done:
                for _, c := range cm.collectors {
                    c.Close()
                }
                cm.wg.Done()
                log.Print("[CollectorManager] DONE\n")
                break CollectorManagerLoop
            case t := <-ticker.C:
                for _, c := range cm.collectors {
CollectorManagerInputLoop:
                    select {
                    case <- cm.done:
                        for _, c := range cm.collectors {
                            c.Close()
                        }
                        cm.wg.Done()
                        log.Print("[CollectorManager] DONE\n")
                        break CollectorManagerInputLoop
                    default:
                        log.Print("[CollectorManager] ", c.Name(), " ", t)
                        c.Read(cm.duration, cm.output)
                    }
                }
            }
        }
        log.Print("[CollectorManager] EXIT\n")
    }()
    log.Print("[CollectorManager] STARTED\n")
}

func (cm *collectorManager) AddOutput(output chan lp.CCMetric) {
    cm.output = output
}

func (cm *collectorManager) Close() {
    cm.done <- true
    log.Print("[CollectorManager] CLOSE")
}

func New(interval time.Duration, duration time.Duration, wg *sync.WaitGroup, collectConfigFile string) (CollectorManager, error) {
    cm := &collectorManager{}
    err := cm.Init(interval, duration, wg, collectConfigFile)
    if err != nil {
        return nil, err
    }
    return cm, err
}
