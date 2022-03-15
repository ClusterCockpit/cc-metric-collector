# CC Metric Router

The CCMetric router sits in between the collectors and the sinks and can be used to add and remove tags to/from traversing [CCMetrics](../ccMetric/README.md).

# Configuration

```json
{
    "num_cache_intervals" : 1,
    "interval_timestamp" : true,
    "normalize_units": true,
    "add_tags" : [
        {
            "key" : "cluster",
            "value" : "testcluster",
            "if" : "*"
        },
        {
            "key" : "test",
            "value" : "testing",
            "if" : "name == 'temp_package_id_0'"
        }
    ],
    "delete_tags" : [
        {
            "key" : "unit",
            "value" : "*",
            "if" : "*"
        }
    ],
    "interval_aggregates" : [
        {
            "name" : "temp_cores_avg",
            "if" : "match('temp_core_%d+', metric.Name())",
            "function" : "avg(values)",
            "tags" : {
                "type" : "node"
            },
            "meta" : {
                "group": "IPMI",
                "unit": "degC",
                "source": "TempCollector"
            }
        }
    ],
    "drop_metrics" : [
        "not_interesting_metric_at_all"
    ],
    "drop_metrics_if" : [
        "match('temp_core_%d+', metric.Name())"
    ],
    "rename_metrics" : {
        "metric_12345" : "mymetric"
    }
}
```

There are three main options `add_tags`, `delete_tags` and `interval_timestamp`. `add_tags` and `delete_tags` are lists consisting of dicts with `key`, `value` and `if`. The `value` can be omitted in the `delete_tags` part as it only uses the `key` for removal. The `interval_timestamp` setting means that a unique timestamp is applied to all metrics traversing the router during an interval.
# The `interval_timestamp` option

The collectors' `Read()` functions are not called simultaneously and therefore the metrics gathered in an interval can have different timestamps. If you want to avoid that and have a common timestamp (the beginning of the interval), set this option to `true` and the MetricRouter sets the time.

# The `num_cache_intervals` option

If the MetricRouter should buffer metrics of intervals in a MetricCache, this option specifies the number of past intervals that should be kept. If `num_cache_intervals = 0`, the cache is disabled. With `num_cache_intervals = 1`, only the metrics of the last interval are buffered.

A `num_cache_intervals > 0` is required to use the `interval_aggregates` option.

# The `rename_metrics` option

In the ClusterCockpit world we specified a set of standard metrics. Since some collectors determine the metric names based on files, execuables and libraries, they might change from system to system (or installation to installtion, OS to OS, ...). In order to get the common names, you can rename incoming metrics before sending them to the sink. If the metric name matches the `oldname`, it is changed to `newname`

```json
{
  "oldname" : "newname",
  "clock_mhz" : "clock"
}
```

# Conditional manipulation of tags (`add_tags` and `del_tags`)

Common config format:
```json
{
    "key" : "test",
    "value" : "testing",
    "if" : "name == 'temp_package_id_0'"
}
```

## The `del_tags` option

The collectors are free to add whatever `key=value` pair to the metric tags (although the usage of tags should be minimized). If you want to delete a tag afterwards, you can do that. When the `if` condition matches on a metric, the `key` is removed from the metric's tags.

If you want to remove a tag for all metrics, use the condition wildcard `*`. The `value` field can be omitted in the `del_tags` case.

Never delete tags:
- `hostname`
- `type`
- `type-id`

## The `add_tags` option

In some cases, metrics should be tagged or an existing tag changed based on some condition. This can be done in the `add_tags` section. When the `if` condition evaluates to `true`, the tag `key` is added or gets changed to the new `value`.

If the CCMetric name is equal to `temp_package_id_0`, it adds an additional tag `test=testing` to the metric.

For this metric, a more useful example would be:

```json
[
  {
    "key" : "type",
    "value" : "socket",
    "if" : "name == 'temp_package_id_0'"
  },
  {
    "key" : "type-id",
    "value" : "0",
    "if" : "name == 'temp_package_id_0'"
  },
]
```

The metric `temp_package_id_0` corresponds to the tempature of the first CPU socket (=package). With the above configuration, the tags would reflect that because commonly the [TempCollector](../../collectors/tempMetric.md) submits only `node` metrics.

In order to match all metrics, you can use `*`, so in order to add a flag per default. This is useful to attached system-specific tags like `cluster=testcluster`:

```json
{
    "key" : "cluster",
    "value" : "testcluster",
    "if" : "*"
}
```

# Dropping metrics

In some cases, you want to drop a metric and don't get it forwarded to the sinks. There are two options based on the required specification:
- Based only on the metric name -> `drop_metrics` section
- An evaluable condition with more overhead -> `drop_metrics_if` section

## The `drop_metrics` section

The argument is a list of metric names. No futher checks are performed, only a comparison of the metric name

```json
{
  "drop_metrics" : [
      "drop_metric_1",
      "drop_metric_2"
  ]
}
```

The example drops all metrics with the name `drop_metric_1` and `drop_metric_2`.

## The `drop_metrics_if` section

This option takes a list of evaluable conditions and performs them one after the other on **all** metrics incoming from the collectors and the metric cache (aka `interval_aggregates`).

```json
{
  "drop_metrics_if" : [
      "match('drop_metric_%d+', name)",
      "match('cpu', type) && type-id == 0"
  ]
}
```
The first line is comparable with the example in `drop_metrics`, it drops all metrics starting with `drop_metric_` and ending with a number. The second line drops all metrics of the first hardware thread (**not** recommended)


# Aggregate metric values of the current interval with the `interval_aggregates` option

**Note:** `interval_aggregates` works only if `num_cache_intervals` > 0

In some cases, you need to derive new metrics based on the metrics arriving during an interval. This can be done in the `interval_aggregates` section. The logic is similar to the other metric manipulation and filtering options. A cache stores all metrics that arrive during an interval. At the beginning of the *next* interval, the list of metrics is submitted to the MetricAggregator. It derives new metrics and submits them back to the MetricRouter, so they are sent in the next interval but have the timestamp of the previous interval beginning.

```json
"interval_aggregates" : [
  {
    "name" : "new_metric_name",
    "if" : "match('sub_metric_%d+', metric.Name())",
    "function" : "avg(values)",
    "tags" : {
      "key" : "value",
      "type" : "node"
    },
    "meta" : {
      "key" : "value",
      "group": "IPMI",
      "unit": "<copy>",
    }
  }
]
```

The above configuration, collects all metric values for metrics evaluating `if` to `true`. Afterwards it calculates the average `avg` of the `values` (list of all metrics' field `value`) and creates a new CCMetric with the name `new_metric_name` and adds the tags in `tags` and the meta information in `meta`. The special value `<copy>` searches the input metrics and copies the value of the first match of `key` to the new CCMetric.

If you are not interested in the input metrics `sub_metric_%d+` at all, you can add the same condition used here to the `drop_metrics_if` section to drop them.

Use cases for `interval_aggregates`:
- Combine multiple metrics of the a collector to a new one like the [MemstatCollector](../../collectors/memstatMetric.md) does it for `mem_used`)):
```json
  {
    "name" : "mem_used",
    "if" : "source == 'MemstatCollector'",
    "function" : "sum(mem_total) - (sum(mem_free) + sum(mem_buffers) + sum(mem_cached))",
    "tags" : {
      "type" : "node"
    },
    "meta" : {
      "group": "<copy>",
      "unit": "<copy>",
      "source": "<copy>"
    }
  }
```
