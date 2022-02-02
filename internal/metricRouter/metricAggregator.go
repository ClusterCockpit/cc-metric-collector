package metricRouter

import (
	"context"
	"fmt"
	"math"
	"os"
	"strings"
	"time"

	cclog "github.com/ClusterCockpit/cc-metric-collector/internal/ccLogger"

	lp "github.com/ClusterCockpit/cc-metric-collector/internal/ccMetric"
	topo "github.com/ClusterCockpit/cc-metric-collector/internal/ccTopology"

	"github.com/PaesslerAG/gval"
)

type metricAggregatorIntervalConfig struct {
	Name      string            `json:"name"`     // Metric name for the new metric
	Function  string            `json:"function"` // Function to apply on the metric
	Condition string            `json:"if"`       // Condition for applying function
	Tags      map[string]string `json:"tags"`     // Tags for the new metric
	Meta      map[string]string `json:"meta"`     // Meta information for the new metric
	gvalCond  gval.Evaluable
	gvalFunc  gval.Evaluable
}

type metricAggregator struct {
	functions []*metricAggregatorIntervalConfig
	constants map[string]interface{}
	language  gval.Language
	output    chan *lp.CCMetric
}

type MetricAggregator interface {
	AddAggregation(name, function, condition string, tags, meta map[string]string) error
	DeleteAggregation(name string) error
	Init(output chan *lp.CCMetric) error
	Eval(starttime time.Time, endtime time.Time, metrics []*lp.CCMetric)
}

var metricCacheLanguage = gval.NewLanguage(
	gval.Base(),
	gval.Function("sum", sumfunc),
	gval.Function("min", minfunc),
	gval.Function("avg", avgfunc),
	gval.Function("mean", avgfunc),
	gval.Function("max", maxfunc),
	gval.Function("len", lenfunc),
	gval.Function("median", medianfunc),
	gval.InfixOperator("in", infunc),
	gval.Function("match", matchfunc),
	gval.Function("getCpuCore", getCpuCoreFunc),
	gval.Function("getCpuSocket", getCpuSocketFunc),
	gval.Function("getCpuNuma", getCpuNumaDomainFunc),
	gval.Function("getCpuDie", getCpuDieFunc),
	gval.Function("getSockCpuList", getCpuListOfSocketFunc),
	gval.Function("getNumaCpuList", getCpuListOfNumaDomainFunc),
	gval.Function("getDieCpuList", getCpuListOfDieFunc),
	gval.Function("getCoreCpuList", getCpuListOfCoreFunc),
	gval.Function("getCpuList", getCpuListOfNode),
	gval.Function("getCpuListOfType", getCpuListOfType),
)

func (c *metricAggregator) Init(output chan *lp.CCMetric) error {
	c.output = output
	c.functions = make([]*metricAggregatorIntervalConfig, 0)
	c.constants = make(map[string]interface{})

	// add constants like hostname, numSockets, ... to constants list
	// Set hostname
	hostname, err := os.Hostname()
	if err != nil {
		cclog.Error(err.Error())
		return err
	}
	// Drop domain part of host name
	c.constants["hostname"] = strings.SplitN(hostname, `.`, 2)[0]
	cinfo := topo.CpuInfo()
	c.constants["numHWThreads"] = cinfo.NumHWthreads
	c.constants["numSockets"] = cinfo.NumSockets
	c.constants["numNumaDomains"] = cinfo.NumNumaDomains
	c.constants["numDies"] = cinfo.NumDies
	c.constants["smtWidth"] = cinfo.SMTWidth

	c.language = gval.NewLanguage(
		gval.Full(),
		metricCacheLanguage,
	)

	// Example aggregation function
	// var f metricCacheFunctionConfig
	// f.Name = "temp_cores_avg"
	// //f.Condition = `"temp_core_" in name`
	// f.Condition = `match("temp_core_%d+", metric.Name())`
	// f.Function = `avg(values)`
	// f.Tags = map[string]string{"type": "node"}
	// f.Meta = map[string]string{"group": "IPMI", "unit": "degC", "source": "TempCollector"}
	// c.functions = append(c.functions, &f)
	return nil
}

func (c *metricAggregator) Eval(starttime time.Time, endtime time.Time, metrics []*lp.CCMetric) {
	vars := make(map[string]interface{})
	for k, v := range c.constants {
		vars[k] = v
	}
	vars["starttime"] = starttime
	vars["endtime"] = endtime
	for _, f := range c.functions {
		cclog.ComponentDebug("MetricCache", "COLLECT", f.Name, "COND", f.Condition)
		values := make([]float64, 0)
		matches := make([]*lp.CCMetric, 0)
		for _, m := range metrics {
			vars["metric"] = *m
			//value, err := gval.Evaluate(f.Condition, vars, c.language)
			value, err := f.gvalCond.EvalBool(context.Background(), vars)
			if err != nil {
				cclog.ComponentError("MetricCache", "COLLECT", f.Name, "COND", f.Condition, ":", err.Error())
				continue
			}
			if value {
				v, valid := (*m).GetField("value")
				if valid {
					switch x := v.(type) {
					case float64:
						values = append(values, x)
					case float32:
					case int:
					case int64:
						values = append(values, float64(x))
					case bool:
						if x {
							values = append(values, float64(1.0))
						} else {
							values = append(values, float64(0.0))
						}
					default:
						cclog.ComponentError("MetricCache", "COLLECT ADD VALUE", v, "FAILED")
					}
				}
				matches = append(matches, m)
			}
		}
		delete(vars, "metric")
		cclog.ComponentDebug("MetricCache", "EVALUATE", f.Name, "METRICS", len(values), "CALC", f.Function)
		vars["values"] = values
		vars["metrics"] = matches
		if len(values) > 0 {
			value, err := gval.Evaluate(f.Function, vars, c.language)
			if err != nil {
				cclog.ComponentError("MetricCache", "EVALUATE", f.Name, "METRICS", len(values), "CALC", f.Function, ":", err.Error())
				break
			}

			copy_tags := func(tags map[string]string, metrics []*lp.CCMetric) map[string]string {
				out := make(map[string]string)
				for key, value := range tags {
					switch value {
					case "<copy>":
						for _, m := range metrics {
							point := *m
							v, err := point.GetTag(key)
							if err {
								out[key] = v
							}
						}
					default:
						out[key] = value
					}
				}
				return out
			}
			copy_meta := func(meta map[string]string, metrics []*lp.CCMetric) map[string]string {
				out := make(map[string]string)
				for key, value := range meta {
					switch value {
					case "<copy>":
						for _, m := range metrics {
							point := *m
							v, err := point.GetMeta(key)
							if err {
								out[key] = v
							}
						}
					default:
						out[key] = value
					}
				}
				return out
			}
			tags := copy_tags(f.Tags, matches)
			meta := copy_meta(f.Meta, matches)

			var m lp.CCMetric
			switch t := value.(type) {
			case float64:
				m, err = lp.New(f.Name, tags, meta, map[string]interface{}{"value": t}, starttime)
			case float32:
				m, err = lp.New(f.Name, tags, meta, map[string]interface{}{"value": t}, starttime)
			case int:
				m, err = lp.New(f.Name, tags, meta, map[string]interface{}{"value": t}, starttime)
			case int64:
				m, err = lp.New(f.Name, tags, meta, map[string]interface{}{"value": t}, starttime)
			case string:
				m, err = lp.New(f.Name, tags, meta, map[string]interface{}{"value": t}, starttime)
			default:
				cclog.ComponentError("MetricCache", "Gval returned invalid type", t, "skipping metric", f.Name)
			}
			if err != nil {
				cclog.ComponentError("MetricCache", "Cannot create metric from Gval result", value, ":", err.Error())
			}
			cclog.ComponentDebug("MetricCache", "SEND", m)
			select {
			case c.output <- &m:
			default:
			}

		}
	}
}

func (c *metricAggregator) AddAggregation(name, function, condition string, tags, meta map[string]string) error {
	// Since "" cannot be used inside of JSON strings, we use '' and replace them here because gval does not like ''
	// but wants ""
	newfunc := strings.ReplaceAll(function, "'", "\"")
	newcond := strings.ReplaceAll(condition, "'", "\"")
	gvalCond, err := gval.Full(metricCacheLanguage).NewEvaluable(newcond)
	if err != nil {
		cclog.ComponentError("MetricAggregator", "Cannot add aggregation, invalid if condition", newcond, ":", err.Error())
		return err
	}
	gvalFunc, err := gval.Full(metricCacheLanguage).NewEvaluable(newfunc)
	if err != nil {
		cclog.ComponentError("MetricAggregator", "Cannot add aggregation, invalid function condition", newfunc, ":", err.Error())
		return err
	}
	for _, agg := range c.functions {
		if agg.Name == name {
			agg.Name = name
			agg.Condition = newcond
			agg.Function = newfunc
			agg.Tags = tags
			agg.Meta = meta
			agg.gvalCond = gvalCond
			agg.gvalFunc = gvalFunc
			return nil
		}
	}
	var agg metricAggregatorIntervalConfig
	agg.Name = name
	agg.Condition = newcond
	agg.gvalCond = gvalCond
	agg.Function = newfunc
	agg.gvalFunc = gvalFunc
	agg.Tags = tags
	agg.Meta = meta
	c.functions = append(c.functions, &agg)
	return nil
}

func (c *metricAggregator) DeleteAggregation(name string) error {
	for i, agg := range c.functions {
		if agg.Name == name {
			copy(c.functions[i:], c.functions[i+1:])
			c.functions[len(c.functions)-1] = nil
			c.functions = c.functions[:len(c.functions)-1]
			return nil
		}
	}
	return fmt.Errorf("no aggregation for metric name %s", name)
}

func (c *metricAggregator) AddConstant(name string, value interface{}) {
	c.constants[name] = value
}

func (c *metricAggregator) DelConstant(name string) {
	delete(c.constants, name)
}

func (c *metricAggregator) AddFunction(name string, function func(args ...interface{}) (interface{}, error)) {
	c.language = gval.NewLanguage(c.language, gval.Function(name, function))
}


func EvalBoolCondition(condition string, params map[string]interface{}) (bool, error) {
	newcond := strings.ReplaceAll(condition, "'", "\"")
	newcond = strings.ReplaceAll(newcond, "%", "\\")
	language := gval.NewLanguage(
		gval.Full(),
		metricCacheLanguage,
	)
	value, err := gval.Evaluate(newcond, params, language)
	if err != nil {
		return false, err
	}
	var endResult bool = false
	err = nil
	switch r := value.(type) {
	case bool:
		endResult = r
	case float64:
		if r != 0.0 {
			endResult = true
		}
	case float32:
		if r != 0.0 {
			endResult = true
		}
	case int:
		if r != 0 {
			endResult = true
		}
	case int64:
		if r != 0 {
			endResult = true
		}
	case int32:
		if r != 0 {
			endResult = true
		}
	default:
		err = fmt.Errorf("cannot evaluate '%s' to bool", newcond)
	}
	return endResult, err
}

func EvalFloat64Condition(condition string, params map[string]interface{}) (float64, error) {
	var endResult float64 = math.NaN()
	newcond := strings.ReplaceAll(condition, "'", "\"")
	newcond = strings.ReplaceAll(newcond, "%", "\\")
	language := gval.NewLanguage(
		gval.Full(),
		metricCacheLanguage,
	)
	value, err := gval.Evaluate(newcond, params, language)
	if err != nil {
		cclog.ComponentDebug("MetricRouter", condition, " = ", err.Error())
		return endResult, err
	}
	err = nil
	switch r := value.(type) {
	case bool:
		if r {
			endResult = 1.0
		} else {
			endResult = 0.0
		}
	case float64:
		endResult = r
	case float32:
		endResult = float64(r)
	case int:
		endResult = float64(r)
	case int64:
		endResult = float64(r)
	case int32:
		endResult = float64(r)
	default:
		err = fmt.Errorf("cannot evaluate '%s' to float64", newcond)
	}
	return endResult, err
}

func NewAggregator(output chan *lp.CCMetric) (MetricAggregator, error) {
	a := new(metricAggregator)
	err := a.Init(output)
	if err != nil {
		return nil, err
	}
	return a, err
}
