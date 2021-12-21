package metricRouter

import (
	"encoding/json"
	lp "github.com/ClusterCockpit/cc-metric-collector/internal/ccMetric"
	mct "github.com/ClusterCockpit/cc-metric-collector/internal/multiChanTicker"
	"gopkg.in/Knetic/govaluate.v2"
	"log"
	"os"
	"sync"
	"time"
)

type metricRounterTagConfig struct {
	Key       string `json:"key"`
	Value     string `json:"value"`
	Condition string `json:"if"`
}

type metricRouterConfig struct {
	AddTags       []metricRounterTagConfig `json:"add_tags"`
	DelTags       []metricRounterTagConfig `json:"delete_tags"`
	IntervalStamp bool                     `json:"interval_timestamp"`
}

type metricRouter struct {
	inputs    []chan lp.CCMetric
	outputs   []chan lp.CCMetric
	done      chan bool
	wg        *sync.WaitGroup
	timestamp time.Time
	ticker    mct.MultiChanTicker
	config    metricRouterConfig
}

type MetricRouter interface {
	Init(ticker mct.MultiChanTicker, wg *sync.WaitGroup, routerConfigFile string) error
	AddInput(input chan lp.CCMetric)
	AddOutput(output chan lp.CCMetric)
	Start()
	Close()
}

func (r *metricRouter) Init(ticker mct.MultiChanTicker, wg *sync.WaitGroup, routerConfigFile string) error {
	r.inputs = make([]chan lp.CCMetric, 0)
	r.outputs = make([]chan lp.CCMetric, 0)
	r.done = make(chan bool)
	r.wg = wg
	r.ticker = ticker
	configFile, err := os.Open(routerConfigFile)
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

func (r *metricRouter) StartTimer() {
	m := make(chan time.Time)
	r.ticker.AddChannel(m)
	go func() {
		for {
			select {
			case t := <-m:
				r.timestamp = t
			}
		}
	}()
}

func (r *metricRouter) EvalCondition(Cond string, point lp.CCMetric) (bool, error) {
	expression, err := govaluate.NewEvaluableExpression(Cond)
	if err != nil {
		log.Print(Cond, " = ", err.Error())
		return false, err
	}
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

	result, err := expression.Evaluate(params)
	if err != nil {
		log.Print(Cond, " = ", err.Error())
		return false, err
	}
	return bool(result.(bool)), err
}

func (r *metricRouter) DoAddTags(point lp.CCMetric) {
	for _, m := range r.config.AddTags {
		var res bool
		var err error

		if m.Condition == "*" {
			res = true
			err = nil
		} else {
			res, err = r.EvalCondition(m.Condition, point)
			if err != nil {
				log.Print(err.Error())
				res = false
			}
		}
		if res == true {
			point.AddTag(m.Key, m.Value)
		}
	}
}

func (r *metricRouter) DoDelTags(point lp.CCMetric) {
	for _, m := range r.config.DelTags {
		var res bool
		var err error
		if m.Condition == "*" {
			res = true
			err = nil
		} else {
			res, err = r.EvalCondition(m.Condition, point)
			if err != nil {
				log.Print(err.Error())
				res = false
			}
		}
		if res == true {
			point.RemoveTag(m.Key)
		}
	}
}

func (r *metricRouter) Start() {
	r.wg.Add(1)
	r.timestamp = time.Now()
	if r.config.IntervalStamp == true {
		r.StartTimer()
	}
	go func() {
		for {
		RouterLoop:
			select {
			case <-r.done:
				log.Print("[MetricRouter] DONE\n")
				r.wg.Done()
				break RouterLoop
			default:
				for _, c := range r.inputs {
				RouterInputLoop:
					select {
					case <-r.done:
						log.Print("[MetricRouter] DONE\n")
						r.wg.Done()
						break RouterInputLoop
					case p := <-c:
						log.Print("[MetricRouter] FORWARD ", p)
						r.DoAddTags(p)
						r.DoDelTags(p)
						if r.config.IntervalStamp == true {
							p.SetTime(r.timestamp)
						}
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

func New(ticker mct.MultiChanTicker, wg *sync.WaitGroup, routerConfigFile string) (MetricRouter, error) {
	r := &metricRouter{}
	err := r.Init(ticker, wg, routerConfigFile)
	if err != nil {
		return nil, err
	}
	return r, err
}
