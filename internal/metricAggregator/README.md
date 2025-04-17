<!--
---
title: Metric Aggregator
description: Subsystem for evaluating expressions on metrics (deprecated)
categories: [cc-metric-collector]
tags: ['Developer']
weight: 1
hugo_path: docs/reference/cc-metric-collector/internal/metricaggregator/_index.md
---
-->

# The MetricAggregator

In some cases, further combination of metrics or raw values is required. For that strings like `foo + 1` with runtime dependent `foo` need to be evaluated. The MetricAggregator relies on the [`gval`](https://github.com/PaesslerAG/gval) Golang package to perform all expression evaluation. The `gval` package provides the basic arithmetic operations but the MetricAggregator defines additional ones.

**Note**: To get an impression which expressions can be handled by `gval`, see its [README](https://github.com/PaesslerAG/gval/blob/master/README.md)

## Simple expression evaluation

For simple expression evaluation, the MetricAggregator provides two function for different use-cases:
- `EvalBoolCondition(expression string, params map[string]interface{}`: Used by the MetricRouter to match metrics like `metric.Name() == 'mymetric'`
- `EvalFloat64Condition(expression string, params map[string]interface{})`: Used by the MetricRouter and LikwidCollector to derive new values like `(PMC0+PMC1)/PMC3`

## MetricAggregator extensions for `gval`

The MetricAggregator provides these functions additional to the `Full` language in `gval`:
- `sum(array)`: Sum up values in an array like `sum(values)`
- `min(array)`: Get the minimum value in an array like `min(values)`
- `avg(array)`: Get the mean value in an array like `avg(values)`
- `mean(array)`: Get the mean value in an array like `mean(values)`
- `max(array)`: Get the maximum value in an array like `max(values)`
- `len(array)`: Get the length of an array like `len(values)`
- `median(array)`: Get the median value in an array like `mean(values)`
- `in`: Check existence in an array like `0 in getCpuList()` to check whether there is an entry `0`. Also substring matching works like `temp in metric.Name()`
- `match`: Regular-expression matching like `match('temp_cores_%d+', metric.Name())`. **Note** all `\` in an regex has to be replaced with `%`
- `getCpuCore(cpuid)`: For a CPU id, the the corresponding CPU core id like `getCpuCore(0)`
- `getCpuSocket(cpuid)`: For a CPU id, the the corresponding CPU socket id
- `getCpuNuma(cpuid)`: For a CPU id, the the corresponding NUMA domain id
- `getCpuDie(cpuid)`: For a CPU id, the the corresponding CPU die id
- `getSockCpuList(sockid)`: For a given CPU socket id, the list of CPU ids is returned like the CPUs on socket 1 `getSockCpuList(1)`
- `getNumaCpuList(numaid)`: For a given NUMA node id, the list of CPU ids is returned
- `getDieCpuList(dieid)`: For a given CPU die id, the list of CPU ids is returned
- `getCoreCpuList(coreid)`: For a given CPU core id, the list of CPU ids is returned
- `getCpuList`: Get the list of all CPUs

## Limitations

- Since the metrics are written in JSON files which do not allow `""` without proper escaping inside of JSON strings, you have to use `''` for strings.
- Since `\` is interpreted by JSON as escape character, it cannot be used in metrics. But it is required to write regular expressions. So instead of `/`, use `%` and the MetricAggregator replaces them after reading the JSON file.
