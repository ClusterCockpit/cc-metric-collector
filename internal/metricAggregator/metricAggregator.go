package metricAggregator

import (
	"context"
	"fmt"
	"math"
	"os"
	"strings"
	"sync"
	"time"

	cclog "github.com/ClusterCockpit/cc-metric-collector/pkg/ccLogger"

	lp "github.com/ClusterCockpit/cc-metric-collector/pkg/ccMetric"
	topo "github.com/ClusterCockpit/cc-metric-collector/pkg/ccTopology"

	"github.com/PaesslerAG/gval"
)

type MetricAggregatorIntervalConfig struct {
	Name      string            `json:"name"`     // Metric name for the new metric
	Function  string            `json:"function"` // Function to apply on the metric
	Condition string            `json:"if"`       // Condition for applying function
	Tags      map[string]string `json:"tags"`     // Tags for the new metric
	Meta      map[string]string `json:"meta"`     // Meta information for the new metric
	gvalCond  gval.Evaluable
	gvalFunc  gval.Evaluable
}

type metricAggregator struct {
	functions []*MetricAggregatorIntervalConfig
	constants map[string]interface{}
	language  gval.Language
	output    chan lp.CCMetric
}

type MetricAggregator interface {
	AddAggregation(name, function, condition string, tags, meta map[string]string) error
	DeleteAggregation(name string) error
	Init(output chan lp.CCMetric) error
	Eval(starttime time.Time, endtime time.Time, metrics []lp.CCMetric)
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
var language gval.Language = gval.NewLanguage(
	gval.Full(),
	metricCacheLanguage,
)
var evaluables = struct {
	mapping map[string]gval.Evaluable
	mutex   sync.Mutex
}{
	mapping: make(map[string]gval.Evaluable),
}

func (c *metricAggregator) Init(output chan lp.CCMetric) error {
	c.output = output
	c.functions = make([]*MetricAggregatorIntervalConfig, 0)
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

func (c *metricAggregator) Eval(starttime time.Time, endtime time.Time, metrics []lp.CCMetric) {
	vars := make(map[string]interface{})
	for k, v := range c.constants {
		vars[k] = v
	}
	vars["starttime"] = starttime
	vars["endtime"] = endtime
	for _, f := range c.functions {
		cclog.ComponentDebug("MetricCache", "COLLECT", f.Name, "COND", f.Condition)
		var valuesFloat64 []float64
		var valuesFloat32 []float32
		var valuesInt []int
		var valuesInt32 []int32
		var valuesInt64 []int64
		var valuesBool []bool
		matches := make([]lp.CCMetric, 0)
		for _, m := range metrics {
			vars["metric"] = m
			//value, err := gval.Evaluate(f.Condition, vars, c.language)
			value, err := f.gvalCond.EvalBool(context.Background(), vars)
			if err != nil {
				cclog.ComponentError("MetricCache", "COLLECT", f.Name, "COND", f.Condition, ":", err.Error())
				continue
			}
			if value {
				v, valid := m.GetField("value")
				if valid {
					switch x := v.(type) {
					case float64:
						valuesFloat64 = append(valuesFloat64, x)
					case float32:
						valuesFloat32 = append(valuesFloat32, x)
					case int:
						valuesInt = append(valuesInt, x)
					case int32:
						valuesInt32 = append(valuesInt32, x)
					case int64:
						valuesInt64 = append(valuesInt64, x)
					case bool:
						valuesBool = append(valuesBool, x)
					default:
						cclog.ComponentError("MetricCache", "COLLECT ADD VALUE", v, "FAILED")
					}
				}
				matches = append(matches, m)
			}
		}
		delete(vars, "metric")

		// Check, that only values of one type were collected
		countValueTypes := 0
		if len(valuesFloat64) > 0 {
			countValueTypes += 1
		}
		if len(valuesFloat32) > 0 {
			countValueTypes += 1
		}
		if len(valuesInt) > 0 {
			countValueTypes += 1
		}
		if len(valuesInt32) > 0 {
			countValueTypes += 1
		}
		if len(valuesInt64) > 0 {
			countValueTypes += 1
		}
		if len(valuesBool) > 0 {
			countValueTypes += 1
		}
		if countValueTypes > 1 {
			cclog.ComponentError("MetricCache", "Collected values of different types")
		}

		var len_values int
		switch {
		case len(valuesFloat64) > 0:
			vars["values"] = valuesFloat64
			len_values = len(valuesFloat64)
		case len(valuesFloat32) > 0:
			vars["values"] = valuesFloat32
			len_values = len(valuesFloat32)
		case len(valuesInt) > 0:
			vars["values"] = valuesInt
			len_values = len(valuesInt)
		case len(valuesInt32) > 0:
			vars["values"] = valuesInt32
			len_values = len(valuesInt32)
		case len(valuesInt64) > 0:
			vars["values"] = valuesInt64
			len_values = len(valuesInt64)
		case len(valuesBool) > 0:
			vars["values"] = valuesBool
			len_values = len(valuesBool)
		}
		cclog.ComponentDebug("MetricCache", "EVALUATE", f.Name, "METRICS", len_values, "CALC", f.Function)

		vars["metrics"] = matches
		if len_values > 0 {
			value, err := gval.Evaluate(f.Function, vars, c.language)
			if err != nil {
				cclog.ComponentError("MetricCache", "EVALUATE", f.Name, "METRICS", len_values, "CALC", f.Function, ":", err.Error())
				break
			}

			copy_tags := func(tags map[string]string, metrics []lp.CCMetric) map[string]string {
				out := make(map[string]string)
				for key, value := range tags {
					switch value {
					case "<copy>":
						for _, m := range metrics {
							v, err := m.GetTag(key)
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
			copy_meta := func(meta map[string]string, metrics []lp.CCMetric) map[string]string {
				out := make(map[string]string)
				for key, value := range meta {
					switch value {
					case "<copy>":
						for _, m := range metrics {
							v, err := m.GetMeta(key)
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
			case c.output <- m:
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
	agg := &MetricAggregatorIntervalConfig{
		Name:      name,
		Condition: newcond,
		gvalCond:  gvalCond,
		Function:  newfunc,
		gvalFunc:  gvalFunc,
		Tags:      tags,
		Meta:      meta,
	}
	c.functions = append(c.functions, agg)
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
	evaluables.mutex.Lock()
	evaluable, ok := evaluables.mapping[condition]
	evaluables.mutex.Unlock()
	if !ok {
		newcond :=
			strings.ReplaceAll(
				strings.ReplaceAll(
					condition, "'", "\""), "%", "\\")
		var err error
		evaluable, err = language.NewEvaluable(newcond)
		if err != nil {
			return false, err
		}
		evaluables.mutex.Lock()
		evaluables.mapping[condition] = evaluable
		evaluables.mutex.Unlock()
	}
	value, err := evaluable.EvalBool(context.Background(), params)
	return value, err
}

func EvalFloat64Condition(condition string, params map[string]interface{}) (float64, error) {
	evaluables.mutex.Lock()
	evaluable, ok := evaluables.mapping[condition]
	evaluables.mutex.Unlock()
	if !ok {
		newcond :=
			strings.ReplaceAll(
				strings.ReplaceAll(
					condition, "'", "\""), "%", "\\")
		var err error
		evaluable, err = language.NewEvaluable(newcond)
		if err != nil {
			return math.NaN(), err
		}
		evaluables.mutex.Lock()
		evaluables.mapping[condition] = evaluable
		evaluables.mutex.Unlock()
	}
	value, err := evaluable.EvalFloat64(context.Background(), params)
	return value, err
}

func NewAggregator(output chan lp.CCMetric) (MetricAggregator, error) {
	a := new(metricAggregator)
	err := a.Init(output)
	if err != nil {
		return nil, err
	}
	return a, err
}
