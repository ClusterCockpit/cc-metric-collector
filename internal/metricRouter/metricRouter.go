package metricRouter

import (
    lp "github.com/ClusterCockpit/cc-metric-collector/internal/ccMetric"
    "sync"
    "log"
    "encoding/json"
    "os"
)

type metricRounterTagConfig struct {
    Key string `json:"key"`
    Value string `json:"value"`
    Condition string `json:"if"`
}

type metricRouterConfig struct {
    AddTags []metricRounterTagConfig `json:"add_tags"`
    DelTags []metricRounterTagConfig `json:"delete_tags"`
    IntervalStamp bool `json:"interval_timestamp"`
}

type metricRouter struct {
	inputs     []chan lp.CCMetric
	outputs     []chan lp.CCMetric
	done        chan bool
	wg          *sync.WaitGroup
	config      metricRouterConfig
}

type MetricRouter interface {
	Init(routerDone chan bool, wg *sync.WaitGroup) error
	AddInput(input chan lp.CCMetric)
	AddOutput(output chan lp.CCMetric)
	ReadConfig(filename string) error
	Start()
	Close()
}


func (r *metricRouter) Init(routerDone chan bool, wg *sync.WaitGroup) error {
    r.inputs = make([]chan lp.CCMetric, 0)
    r.outputs = make([]chan lp.CCMetric, 0)
    r.done = routerDone
    r.wg = wg
    return nil
}

func (r *metricRouter) ReadConfig(filename string) error {
    configFile, err := os.Open(filename)
    if err != nil {
        log.Print(err.Error())
        return err
    }
    defer configFile.Close()
    jsonParser := json.NewDecoder(configFile)
	err = jsonParser.Decode(&r.config)
	if err != nil {
	    log.Print(err.Error())
        return err
    }
    return nil
}

func (r *metricRouter) Start() {
    r.wg.Add(1)
    go func() {
        for {
RouterLoop:
            select {
            case <- r.done:
                log.Print("[MetricRouter] DONE\n")
                r.wg.Done()
                break RouterLoop
            default:
                for _, c := range r.inputs {
RouterInputLoop:
                    select {
                    case <- r.done:
                        log.Print("[MetricRouter] DONE\n")
                        r.wg.Done()
                        break RouterInputLoop
                    case p := <- c:
                        log.Print("[MetricRouter] FORWARD ",p)
                        for _, o := range r.outputs {
                            o <- p
                        }
                    default:
                    }
                }
            }
        }
        log.Print("[MetricRouter] EXIT\n")
    }()
    log.Print("[MetricRouter] STARTED\n")
}

func (r *metricRouter) AddInput(input chan lp.CCMetric) {
    r.inputs = append(r.inputs, input)
}

func (r *metricRouter) AddOutput(output chan lp.CCMetric) {
    r.outputs = append(r.outputs, output)
}

func (r *metricRouter) Close() {
    r.done <- true
    log.Print("[MetricRouter] CLOSE\n")
}

func New(done chan bool, wg *sync.WaitGroup) (MetricRouter, error) {
    r := &metricRouter{}
    err := r.Init(done, wg)
    if err != nil {
        return nil, err
    }
    return r, err
}
