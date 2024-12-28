# PerfCmdMetric collector


## Configuration

```json
{
    "perf_command": "perf",
    "metrics" : [
        {
            "name": "cpu_cycles",
            "event": "cycles",
            "unit": "Hz",
            "type": "hwthread",
            "publish": true,
            "use_perf_unit": false,
            "type_aggregation": "socket",
            "tags": {
                "tags_just" : "for_the_event"
            },
            "meta": {
                "meta_info_just" : "for_the_event"
            }
        }
    ],
    "expressions": [
        {
            "metric": "avg_cycles_per_second",
            "expression": "cpu_cycles / time",
            "type": "node",
            "type_aggregation": "avg",
            "publish": true
        }
    ]
}
```

- `perf_command`: Path to the `perf` command. If it is not an absolute path, the command is looked up in `$PATH`.
- `metrics`: List of metrics to measure
    - `name`: Name of metric for output and expressions
    - `event`: Event as supplied to `perf stat -e <event>` like `cycles` or `uncore_imc_0/event=0x01,umask=0x00/`
    - `unit` : Unit for the metric. Will be added as meta information thus similar then adding `"meta" : {"unit": "myunit"}`.
    - `type`: Do measurments at this level (`hwthread` and `socket` are the most common ones).
    - `publish`: Publish the metric or use it only for expressions.
    - `use_perf_unit`: For some events, `perf` outputs a unit. With this switch, the unit provided by `perf` is added as meta informations.
    - `type_aggregation`: Sum the metric values to the given type
    - `tags`: Tags just for this metric
    - `meta`: Meta informations just for this metric
- `expressions`: Calculate metrics out of multiple measurements
    - `metric`: Name of metric for output
    - `expression`: What should be calculated
    - `type`: Aggregate the expression results to this level
    - `type_aggregation`: Aggregate the expression results with `sum`, `min`, `max`, `avg` or `mean`
    - `publish`: Publish metric