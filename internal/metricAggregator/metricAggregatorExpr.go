package metricAggregator

import (
	"fmt"
	"maps"
	"math"
	"slices"
	"strings"
	"sync"
	"time"

	cclog "github.com/ClusterCockpit/cc-lib/v2/ccLogger"
	lp "github.com/ClusterCockpit/cc-lib/v2/ccMessage"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
)

type MetricAggregatorExprIntervalConfig struct {
	Name                     string            `json:"name"`           // Metric name for the new metric
	Function                 string            `json:"function"`       // Function to apply on the metric
	Condition                string            `json:"if"`             // Condition for applying function
	Tags                     map[string]string `json:"tags,omitempty"` // Tags for the new metric
	Meta                     map[string]string `json:"meta,omitempty"` // Meta information for the new metric
	ValueType                string            `json:"value_type,omitempty"`
	IncludeNumCacheIntervals int               `json:"expr_on_num_intervals,omitempty"`
	exprCond                 *vm.Program
	exprFunc                 *vm.Program
}

type metricAggregatorExpr struct {
	constants    map[string]any
	output       chan lp.CCMessage
	aggregations []MetricAggregatorExprIntervalConfig
}

var paramMapPool = sync.Pool{
	New: func() any {
		return make(map[string]any)
	},
}

func sanitizeExprString(key string) string {
	return strings.ReplaceAll(key, "type-id", "typeid")
}

// AddAggregation(name, function, condition string, tags, meta map[string]string) error
// 	DeleteAggregation(name string) error
// 	Init(output chan lp.CCMessage) error
// 	Eval(starttime time.Time, endtime time.Time, metrics []lp.CCMessage)

func (m *metricAggregatorExpr) Init(output chan lp.CCMessage) error {
	m.output = output
	m.constants = make(map[string]any)
	m.aggregations = make([]MetricAggregatorExprIntervalConfig, 0)
	return nil
}

func GetParamMap(point lp.CCMessage) map[string]any {
	params := paramMapPool.Get().(map[string]any)
	clear(params)

	// Put metric name into params map
	params["name"] = point.Name()

	// Put full message into params map
	params["message"] = point
	params["msg"] = point

	// Put timestamp into params map
	params["timestamp"] = point.Time().Unix()
	params["time"] = params["timestamp"]

	// Put fields into params map
	fields := paramMapPool.Get().(map[string]any)
	clear(fields)
	for key, value := range point.Fields() {
		fields[key] = value
		switch key {
		case "value":
			params["messagetype"] = "metric"
			params["value"] = value
			params["metric"] = value
		case "event":
			params["messagetype"] = "event"
			params["event"] = value
		case "control":
			params["messagetype"] = "control"
			params["control"] = value
		case "log":
			params["messagetype"] = "log"
			params["log"] = value
		default:
			params["messagetype"] = "unknown"
		}
	}
	params["msgtype"] = params["messagetype"]
	params["fields"] = fields
	params["field"] = fields

	// Put tags into params map
	tags := paramMapPool.Get().(map[string]any)
	clear(tags)
	for key, value := range point.Tags() {
		tags[sanitizeExprString(key)] = value
	}
	params["tags"] = tags
	params["tag"] = tags

	// Put meta information into params map
	meta := paramMapPool.Get().(map[string]any)
	clear(meta)
	for key, value := range point.Meta() {
		meta[sanitizeExprString(key)] = value
	}
	params["meta"] = meta

	return params
}

var baseenv_multi_message = map[string]any{
	"name":                   "",
	"starttime":              1234567890,
	"endtime":                1234567890,
	"messages":               make([]lp.CCMessage, 0),
	"Median":                 medianfunc,
	"Sum":                    sumfunc,
	"Min":                    minfunc,
	"Max":                    maxfunc,
	"Mean":                   avgfunc,
	"Avg":                    avgfunc,
	"Match":                  matchfunc,
	"getCpuCore":             getCpuCoreFunc,
	"getCpuSocket":           getCpuSocketFunc,
	"getCpuNumaDomain":       getCpuNumaDomainFunc,
	"getCpuDie":              getCpuDieFunc,
	"getCpuListOfCore":       getCpuListOfCoreFunc,
	"getCpuListOfSocket":     getCpuListOfSocketFunc,
	"getCpuListOfNumaDomain": getCpuListOfNumaDomainFunc,
	"getCpuListOfDie":        getCpuListOfDieFunc,
	"getCpuListOfNode":       getCpuListOfNodeFunc,
	"getCpuListOfType":       getCpuListOfTypeFunc,
}

func (m *metricAggregatorExpr) AddAggregationWithType(name, function, condition, valueType string, tags, meta map[string]string) error {

	cond, err := expr.Compile(condition, expr.Env(baseenv_multi_message), expr.AsBool(), expr.AllowUndefinedVariables())
	if err != nil {
		err = fmt.Errorf("failed to compile condition for aggregation %s: %s", name, err.Error())
		return err
	}
	var exprOption expr.Option = expr.AsFloat64()
	switch valueType {
	case "int":
	case "int32":
		exprOption = expr.AsInt()
	case "int64":
		exprOption = expr.AsInt64()
	case "float32":
	case "float64":
		exprOption = expr.AsFloat64()
	case "bool":
		exprOption = expr.AsBool()
	default:
		err := fmt.Errorf("invalid value type '%s' for aggregation %s", valueType, name)
		return err
	}
	f, err := expr.Compile(function, expr.Env(baseenv_multi_message), exprOption, expr.AllowUndefinedVariables())
	if err != nil {
		err = fmt.Errorf("failed to compile function for aggregation %s: %s", name, err.Error())
		return err
	}
	m.aggregations = append(m.aggregations, MetricAggregatorExprIntervalConfig{
		Name:      name,
		Condition: condition,
		Function:  function,
		Tags:      tags,
		Meta:      meta,
		ValueType: valueType,
		exprCond:  cond,
		exprFunc:  f,
	})
	return nil
}

func (m *metricAggregatorExpr) AddAggregation(name, function, condition string, tags, meta map[string]string) error {
	cclog.ComponentDebugf("MetricAggregator", "Adding %s", name)
	err := m.AddAggregationWithType(name, function, condition, "float64", tags, meta)
	cclog.ComponentDebugf("MetricAggregator", "Adding %s returned %v", name, err)
	return err
}

func (m *metricAggregatorExpr) Eval(starttime, endtime time.Time, metrics []lp.CCMessage) {
	cclog.ComponentDebugf("MetricAggregator", "Calculating %d expressions", len(m.aggregations))
	copy_tags := func(tags map[string]string, metrics []lp.CCMessage) map[string]string {
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
	copy_meta := func(meta map[string]string, metrics []lp.CCMessage) map[string]string {
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

	for _, aggr := range m.aggregations {
		selected_metrics := make([]lp.CCMessage, 0)
		values := make([]float64, 0)
		aggr_vars := make(map[string]any)
		maps.Copy(aggr_vars, baseenv_multi_message)
		maps.Copy(aggr_vars, m.constants)
		aggr_vars["starttime"] = starttime
		aggr_vars["endtime"] = endtime

		for _, met := range metrics {
			met_vars := make(map[string]any)
			maps.Copy(met_vars, aggr_vars)
			met_vars["message"] = met
			maps.Copy(met_vars, GetParamMap(met))
			res, err := expr.Run(aggr.exprCond, met_vars)
			if err != nil || res == false {
				continue
			}
			if value, ok := met.GetField("value"); ok {
				switch v := value.(type) {
				case float64:
					values = append(values, v)
				case float32:
				case int:
				case int8:
				case int16:
				case int32:
				case int64:
				case uint:
				case uint8:
				case uint16:
				case uint32:
				case uint64:
					values = append(values, float64(v))
				case bool:
					if v {
						values = append(values, float64(1))
					} else {
						values = append(values, float64(0))
					}
				default:
					cclog.ComponentErrorf("MetricAggregator", "Cannot convert value type for %s", met.ToLineProtocol(nil))
					continue
				}
				selected_metrics = append(selected_metrics, met)
			}
		}
		cclog.ComponentDebugf("MetricAggregator", "Collected %d values from %d metrics", len(values), len(selected_metrics))
		aggr_vars["values"] = values
		aggr_vars["messages"] = selected_metrics
		if len(values) > 0 && len(selected_metrics) > 0 {
			res, err := expr.Run(aggr.exprFunc, aggr_vars)
			if err == nil {
				tags := copy_tags(aggr.Tags, selected_metrics)
				meta := copy_meta(aggr.Meta, selected_metrics)
				msg, err := lp.NewMetric(aggr.Name, tags, meta, res, time.Now())
				if err == nil {
					cclog.ComponentDebugf("MetricAggregator", "Sending %s", msg.ToLineProtocol(nil))
					select {
					case m.output <- msg:
					default:
					}

				}
			} else {
				cclog.ComponentErrorf("MetricAggregator", "Failed to calculate aggregation with name %s: %s", aggr.Name, err.Error())
			}
		}
	}

}

func (c *metricAggregatorExpr) AddConstant(name string, value any) {
	c.constants[name] = value
}

func (c *metricAggregatorExpr) DelConstant(name string) {
	delete(c.constants, name)
}

func (c *metricAggregatorExpr) DeleteAggregation(name string) error {
	i := slices.IndexFunc(
		c.aggregations,
		func(agg MetricAggregatorExprIntervalConfig) bool {
			return agg.Name == name
		})
	if i == -1 {
		return fmt.Errorf("no aggregation for metric name %s", name)
	}
	copy(c.aggregations[i:], c.aggregations[i+1:])
	c.aggregations = c.aggregations[:len(c.aggregations)-1]
	return nil
}

func NewAggregatorExpr(output chan lp.CCMessage) (MetricAggregator, error) {
	a := new(metricAggregatorExpr)
	err := a.Init(output)
	if err != nil {
		return nil, err
	}
	return a, err
}

var expr_cached map[string]*vm.Program = make(map[string]*vm.Program)
var expr_cached_lock sync.Mutex

var baseenv_message = map[string]any{
	"name":        "",
	"messagetype": "unknown",
	"msgtype":     "unknown",
	"tag": map[string]any{
		"type":     "unknown",
		"typeid":   "0",
		"stype":    "unknown",
		"stypeid":  "0",
		"hostname": "localhost",
		"cluster":  "nocluster",
	},
	"tags": map[string]any{
		"type":     "unknown",
		"typeid":   "0",
		"stype":    "unknown",
		"stypeid":  "0",
		"hostname": "localhost",
		"cluster":  "nocluster",
	},
	"meta": map[string]any{
		"unit":   "invalid",
		"source": "unknown",
	},
	"fields": map[string]any{
		"value":   0,
		"event":   "",
		"control": "",
		"log":     "",
	},
	"field": map[string]any{
		"value":   0,
		"event":   "",
		"control": "",
		"log":     "",
	},
	"timestamp": 1234567890,
	"msg":       lp.EmptyMessage(),
	"message":   lp.EmptyMessage(),
}

func EvalBoolConditionExpr(condition string, msg lp.CCMessage) (bool, error) {
	scond := sanitizeExprString(condition)
	expr_cached_lock.Lock()
	evaluable, ok := expr_cached[scond]
	expr_cached_lock.Unlock()
	if !ok {
		newcond := strings.ReplaceAll(
			strings.ReplaceAll(
				scond, "'", "\""), "%", "\\")
		var err error
		evaluable, err = expr.Compile(newcond, expr.Env(baseenv_message), expr.AsBool())
		if err != nil {
			return false, err
		}
		expr_cached_lock.Lock()
		expr_cached[scond] = evaluable
		expr_cached_lock.Unlock()
	}
	vars := GetParamMap(msg)

	res, err := expr.Run(evaluable, vars)
	if err != nil {
		return false, err
	}
	paramMapPool.Put(vars)
	return res.(bool), nil
}

func EvalFloat64ConditionExpr(condition string, msg lp.CCMessage) (float64, error) {
	scond := sanitizeExprString(condition)
	expr_cached_lock.Lock()
	evaluable, ok := expr_cached[scond]
	expr_cached_lock.Unlock()
	if !ok {
		newcond := strings.ReplaceAll(
			strings.ReplaceAll(
				scond, "'", "\""), "%", "\\")
		var err error
		evaluable, err = expr.Compile(newcond, expr.Env(baseenv_message), expr.AsFloat64())
		if err != nil {
			return math.NaN(), err
		}
		expr_cached_lock.Lock()
		expr_cached[scond] = evaluable
		expr_cached_lock.Unlock()
	}
	vars := GetParamMap(msg)
	res, err := expr.Run(evaluable, vars)
	paramMapPool.Put(vars)
	return res.(float64), err
}

func EvalFloat64Expression(expression string, values map[string]any) (float64, error) {
	sexpr := sanitizeExprString(expression)
	expr_cached_lock.Lock()
	evaluable, ok := expr_cached[sexpr]
	expr_cached_lock.Unlock()
	if !ok {
		newcond := strings.ReplaceAll(
			strings.ReplaceAll(
				sexpr, "'", "\""), "%", "\\")
		var err error
		evaluable, err = expr.Compile(newcond, expr.Env(baseenv_message), expr.AsFloat64())
		if err != nil {
			return math.NaN(), err
		}
		expr_cached_lock.Lock()
		expr_cached[sexpr] = evaluable
		expr_cached_lock.Unlock()
	}
	res, err := expr.Run(evaluable, values)
	return res.(float64), err
}
