package metricAggregator

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"golang.org/x/exp/slices"

	topo "github.com/ClusterCockpit/cc-metric-collector/pkg/ccTopology"
)

/*
 * Arithmetic functions on value arrays
 */

func sumAnyType[T float64 | float32 | int | int32 | int64](values []T) (T, error) {
	if len(values) == 0 {
		return 0.0, errors.New("sum function requires at least one argument")
	}
	var sum T
	for _, value := range values {
		sum += value
	}
	return sum, nil
}

// Sum up values
func sumfunc(args interface{}) (interface{}, error) {

	var err error
	switch values := args.(type) {
	case []float64:
		return sumAnyType(values)
	case []float32:
		return sumAnyType(values)
	case []int:
		return sumAnyType(values)
	case []int64:
		return sumAnyType(values)
	case []int32:
		return sumAnyType(values)
	default:
		err = errors.New("function 'sum' only on list of values (float64, float32, int, int32, int64)")
	}

	return 0.0, err
}

func minAnyType[T float64 | float32 | int | int32 | int64](values []T) (T, error) {
	if len(values) == 0 {
		return 0.0, errors.New("min function requires at least one argument")
	}
	return slices.Min(values), nil
}

// Get the minimum value
func minfunc(args interface{}) (interface{}, error) {
	switch values := args.(type) {
	case []float64:
		return minAnyType(values)
	case []float32:
		return minAnyType(values)
	case []int:
		return minAnyType(values)
	case []int64:
		return minAnyType(values)
	case []int32:
		return minAnyType(values)
	default:
		return 0.0, errors.New("function 'min' only on list of values (float64, float32, int, int32, int64)")
	}
}

func avgAnyType[T float64 | float32 | int | int32 | int64](values []T) (float64, error) {
	if len(values) == 0 {
		return 0.0, errors.New("average function requires at least one argument")
	}
	sum, err := sumAnyType[T](values)
	return float64(sum) / float64(len(values)), err
}

// Get the average or mean value
func avgfunc(args interface{}) (interface{}, error) {
	switch values := args.(type) {
	case []float64:
		return avgAnyType(values)
	case []float32:
		return avgAnyType(values)
	case []int:
		return avgAnyType(values)
	case []int64:
		return avgAnyType(values)
	case []int32:
		return avgAnyType(values)
	default:
		return 0.0, errors.New("function 'average' only on list of values (float64, float32, int, int32, int64)")
	}
}

func maxAnyType[T float64 | float32 | int | int32 | int64](values []T) (T, error) {
	if len(values) == 0 {
		return 0.0, errors.New("max function requires at least one argument")
	}
	return slices.Max(values), nil
}

// Get the maximum value
func maxfunc(args interface{}) (interface{}, error) {
	switch values := args.(type) {
	case []float64:
		return maxAnyType(values)
	case []float32:
		return maxAnyType(values)
	case []int:
		return maxAnyType(values)
	case []int64:
		return maxAnyType(values)
	case []int32:
		return maxAnyType(values)
	default:
		return 0.0, errors.New("function 'max' only on list of values (float64, float32, int, int32, int64)")
	}
}

func medianAnyType[T float64 | float32 | int | int32 | int64](values []T) (T, error) {
	if len(values) == 0 {
		return 0.0, errors.New("median function requires at least one argument")
	}
	slices.Sort(values)
	var median T
	if midPoint := len(values) % 2; midPoint == 0 {
		median = (values[midPoint-1] + values[midPoint]) / 2
	} else {
		median = values[midPoint]
	}
	return median, nil
}

// Get the median value
func medianfunc(args interface{}) (interface{}, error) {
	switch values := args.(type) {
	case []float64:
		return medianAnyType(values)
	case []float32:
		return medianAnyType(values)
	case []int:
		return medianAnyType(values)
	case []int64:
		return medianAnyType(values)
	case []int32:
		return medianAnyType(values)
	default:
		return 0.0, errors.New("function 'median' only on list of values (float64, float32, int, int32, int64)")
	}
}

/*
 * Get number of values in list. Returns always an int
 */

func lenfunc(args interface{}) (interface{}, error) {
	var err error = nil
	var length int = 0
	switch values := args.(type) {
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
func getCpuCoreFunc(args interface{}) (interface{}, error) {
	switch cpuid := args.(type) {
	case int:
		return topo.GetHwthreadCore(cpuid), nil
	}
	return -1, errors.New("function 'getCpuCore' accepts only an 'int' cpuid")
}

// for a given cpuid, it returns the socket id
func getCpuSocketFunc(args interface{}) (interface{}, error) {
	switch cpuid := args.(type) {
	case int:
		return topo.GetHwthreadSocket(cpuid), nil
	}
	return -1, errors.New("function 'getCpuCore' accepts only an 'int' cpuid")
}

// for a given cpuid, it returns the id of the NUMA node
func getCpuNumaDomainFunc(args interface{}) (interface{}, error) {
	switch cpuid := args.(type) {
	case int:
		return topo.GetHwthreadNumaDomain(cpuid), nil
	}
	return -1, errors.New("function 'getCpuNuma' accepts only an 'int' cpuid")
}

// for a given cpuid, it returns the id of the CPU die
func getCpuDieFunc(args interface{}) (interface{}, error) {
	switch cpuid := args.(type) {
	case int:
		return topo.GetHwthreadDie(cpuid), nil
	}
	return -1, errors.New("function 'getCpuDie' accepts only an 'int' cpuid")
}

// for a given core id, it returns the list of cpuids
func getCpuListOfCoreFunc(args interface{}) (interface{}, error) {
	cpulist := make([]int, 0)
	switch in := args.(type) {
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
func getCpuListOfSocketFunc(args interface{}) (interface{}, error) {
	cpulist := make([]int, 0)
	switch in := args.(type) {
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
func getCpuListOfNumaDomainFunc(args interface{}) (interface{}, error) {
	cpulist := make([]int, 0)
	switch in := args.(type) {
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
func getCpuListOfDieFunc(args interface{}) (interface{}, error) {
	cpulist := make([]int, 0)
	switch in := args.(type) {
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
func getCpuListOfNode() (interface{}, error) {
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
