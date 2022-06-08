package metricAggregator

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"

	cclog "github.com/ClusterCockpit/cc-metric-collector/internal/ccLogger"
	topo "github.com/ClusterCockpit/cc-metric-collector/internal/ccTopology"
)

/*
 * Arithmetic functions on value arrays
 */

// Sum up values
func sumfunc(args ...interface{}) (interface{}, error) {
	s := 0.0
	values, ok := args[0].([]float64)
	if ok {
		cclog.ComponentDebug("MetricCache", "SUM FUNC START")
		for _, x := range values {
			s += x
		}
		cclog.ComponentDebug("MetricCache", "SUM FUNC END", s)
	} else {
		cclog.ComponentDebug("MetricCache", "SUM FUNC CAST FAILED")
	}
	return s, nil
}

// Get the minimum value
func minfunc(args ...interface{}) (interface{}, error) {
	var err error = nil
	switch values := args[0].(type) {
	case []float64:
		var s float64 = math.MaxFloat64
		for _, x := range values {
			if x < s {
				s = x
			}
		}
		return s, nil
	case []float32:
		var s float32 = math.MaxFloat32
		for _, x := range values {
			if x < s {
				s = x
			}
		}
		return s, nil
	case []int:
		var s int = int(math.MaxInt32)
		for _, x := range values {
			if x < s {
				s = x
			}
		}
		return s, nil
	case []int64:
		var s int64 = math.MaxInt64
		for _, x := range values {
			if x < s {
				s = x
			}
		}
		return s, nil
	case []int32:
		var s int32 = math.MaxInt32
		for _, x := range values {
			if x < s {
				s = x
			}
		}
		return s, nil
	default:
		err = errors.New("function 'min' only on list of values (float64, float32, int, int32, int64)")
	}

	return 0.0, err
}

// Get the average or mean value
func avgfunc(args ...interface{}) (interface{}, error) {
	switch values := args[0].(type) {
	case []float64:
		var s float64 = 0
		for _, x := range values {
			s += x
		}
		return s / float64(len(values)), nil
	case []float32:
		var s float32 = 0
		for _, x := range values {
			s += x
		}
		return s / float32(len(values)), nil
	case []int:
		var s int = 0
		for _, x := range values {
			s += x
		}
		return s / len(values), nil
	case []int64:
		var s int64 = 0
		for _, x := range values {
			s += x
		}
		return s / int64(len(values)), nil
	}
	return 0.0, nil
}

// Get the maximum value
func maxfunc(args ...interface{}) (interface{}, error) {
	s := 0.0
	values, ok := args[0].([]float64)
	if ok {
		for _, x := range values {
			if x > s {
				s = x
			}
		}
	}
	return s, nil
}

// Get the median value
func medianfunc(args ...interface{}) (interface{}, error) {
	switch values := args[0].(type) {
	case []float64:
		sort.Float64s(values)
		return values[len(values)/2], nil
	// case []float32:
	// 	sort.Float64s(values)
	// 	return values[len(values)/2], nil
	case []int:
		sort.Ints(values)
		return values[len(values)/2], nil

		// case []int64:
		// 	sort.Ints(values)
		// 	return values[len(values)/2], nil
		// case []int32:
		// 	sort.Ints(values)
		// 	return values[len(values)/2], nil
	}
	return 0.0, errors.New("function 'median()' only on lists of type float64 and int")
}

/*
 * Get number of values in list. Returns always an int
 */

func lenfunc(args ...interface{}) (interface{}, error) {
	var err error = nil
	var length int = 0
	switch values := args[0].(type) {
	case []float64:
		length = len(values)
	case []float32:
		length = len(values)
	case []int:
		length = len(values)
	case []int64:
		length = len(values)
	case []int32:
		length = len(values)
	case float64:
		err = errors.New("function 'len' can only be applied on arrays and strings")
	case float32:
		err = errors.New("function 'len' can only be applied on arrays and strings")
	case int:
		err = errors.New("function 'len' can only be applied on arrays and strings")
	case int64:
		err = errors.New("function 'len' can only be applied on arrays and strings")
	case string:
		length = len(values)
	}
	return length, err
}

/*
 * Check if a values is in a list
 * In constrast to most of the other functions, this one is an infix operator for
 * - substring matching: `"abc" in "abcdef"` -> true
 * - substring matching with int casting: `3 in "abd3"` -> true
 * - search for an int in an int list: `3 in getCpuList()` -> true (if you have more than 4 CPU hardware threads)
 */

func infunc(a interface{}, b interface{}) (interface{}, error) {
	switch match := a.(type) {
	case string:
		switch total := b.(type) {
		case string:
			return strings.Contains(total, match), nil
		}
	case int:
		switch total := b.(type) {
		case []int:
			for _, x := range total {
				if x == match {
					return true, nil
				}
			}
		case string:
			smatch := fmt.Sprintf("%d", match)
			return strings.Contains(total, smatch), nil
		}

	}
	return false, nil
}

/*
 * Regex matching of strings (metric name, tag keys, tag values, meta keys, meta values)
 * Since we cannot use \ inside JSON strings without escaping, we use % instead for the
 * format keys \d = %d, \w = %d, ... Not sure how to fix this
 */

func matchfunc(args ...interface{}) (interface{}, error) {
	switch match := args[0].(type) {
	case string:
		switch total := args[1].(type) {
		case string:
			smatch := strings.Replace(match, "%", "\\", -1)
			regex, err := regexp.Compile(smatch)
			if err != nil {
				return false, err
			}
			s := regex.Find([]byte(total))
			return s != nil, nil
		}
	}
	return false, nil
}

/*
 * System topology getter functions
 */

// for a given cpuid, it returns the core id
func getCpuCoreFunc(args ...interface{}) (interface{}, error) {
	switch cpuid := args[0].(type) {
	case int:
		return topo.GetHwthreadCore(cpuid), nil
	}
	return -1, errors.New("function 'getCpuCore' accepts only an 'int' cpuid")
}

// for a given cpuid, it returns the socket id
func getCpuSocketFunc(args ...interface{}) (interface{}, error) {
	switch cpuid := args[0].(type) {
	case int:
		return topo.GetHwthreadSocket(cpuid), nil
	}
	return -1, errors.New("function 'getCpuCore' accepts only an 'int' cpuid")
}

// for a given cpuid, it returns the id of the NUMA node
func getCpuNumaDomainFunc(args ...interface{}) (interface{}, error) {
	switch cpuid := args[0].(type) {
	case int:
		return topo.GetHwthreadNumaDomain(cpuid), nil
	}
	return -1, errors.New("function 'getCpuNuma' accepts only an 'int' cpuid")
}

// for a given cpuid, it returns the id of the CPU die
func getCpuDieFunc(args ...interface{}) (interface{}, error) {
	switch cpuid := args[0].(type) {
	case int:
		return topo.GetHwthreadDie(cpuid), nil
	}
	return -1, errors.New("function 'getCpuDie' accepts only an 'int' cpuid")
}

// for a given core id, it returns the list of cpuids
func getCpuListOfCoreFunc(args ...interface{}) (interface{}, error) {
	cpulist := make([]int, 0)
	switch in := args[0].(type) {
	case int:
		for _, c := range topo.CpuData() {
			if c.Core == in {
				cpulist = append(cpulist, c.Cpuid)
			}
		}
	}
	return cpulist, nil
}

// for a given socket id, it returns the list of cpuids
func getCpuListOfSocketFunc(args ...interface{}) (interface{}, error) {
	cpulist := make([]int, 0)
	switch in := args[0].(type) {
	case int:
		for _, c := range topo.CpuData() {
			if c.Socket == in {
				cpulist = append(cpulist, c.Cpuid)
			}
		}
	}
	return cpulist, nil
}

// for a given id of a NUMA domain, it returns the list of cpuids
func getCpuListOfNumaDomainFunc(args ...interface{}) (interface{}, error) {
	cpulist := make([]int, 0)
	switch in := args[0].(type) {
	case int:
		for _, c := range topo.CpuData() {
			if c.Numadomain == in {
				cpulist = append(cpulist, c.Cpuid)
			}
		}
	}
	return cpulist, nil
}

// for a given CPU die id, it returns the list of cpuids
func getCpuListOfDieFunc(args ...interface{}) (interface{}, error) {
	cpulist := make([]int, 0)
	switch in := args[0].(type) {
	case int:
		for _, c := range topo.CpuData() {
			if c.Die == in {
				cpulist = append(cpulist, c.Cpuid)
			}
		}
	}
	return cpulist, nil
}

// wrapper function to get a list of all cpuids of the node
func getCpuListOfNode(args ...interface{}) (interface{}, error) {
	return topo.HwthreadList(), nil
}

// helper function to get the cpuid list for a CCMetric type tag set (type and type-id)
// since there is no access to the metric data in the function, is should be called like
// `getCpuListOfType()`
func getCpuListOfType(args ...interface{}) (interface{}, error) {
	cpulist := make([]int, 0)
	switch typ := args[0].(type) {
	case string:
		switch typ {
		case "node":
			return topo.HwthreadList(), nil
		case "socket":
			return getCpuListOfSocketFunc(args[1])
		case "numadomain":
			return getCpuListOfNumaDomainFunc(args[1])
		case "core":
			return getCpuListOfCoreFunc(args[1])
		case "hwthread":
			var cpu int

			switch id := args[1].(type) {
			case string:
				_, err := fmt.Scanf(id, "%d", &cpu)
				if err == nil {
					cpulist = append(cpulist, cpu)
				}
			case int:
				cpulist = append(cpulist, id)
			case int64:
				cpulist = append(cpulist, int(id))
			}

		}
	}
	return cpulist, errors.New("no valid args type and type-id")
}
