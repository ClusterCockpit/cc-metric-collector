
## `memstat` collector

```json
  "memstat": {
    "node_stats" : true,
    "numa_stats" : false,
    "exclude_metrics": [
      "mem_used"
    ]
  }
```

By default, the `memstat` collector reads data from `/proc/meminfo` and outputs a handful **node** metrics. This can be deactivated by the `node_stats` option.

Additionally, the `memstat` collector can read the NUMA node specific `/sys/devices/system/node/node*/meminfo` and output them as **memoryDomain** metrics. This can be de/activeate with the `numa_stats` option.

If a metric is not required, it can be excluded from forwarding it to the sink. This includes the metric for system-wide memory stats as well as NUMA node specific memory stats. If you want to filter only specific metrics, use the [MetricRouter](../internal/metricRouter/README.md) with something like:
`name == '<metric_that_should_be_dropped>' && type == 'node'` to keep the NUMA node specific `<metric_that_should_be_dropped>` while dropping the system-wide one.


Metrics:
* `mem_total`
* `mem_sreclaimable`
* `mem_slab`
* `mem_free`
* `mem_buffers`
* `mem_cached`
* `mem_available`
* `mem_shared`
* `swap_total`
* `swap_free`
* `mem_used` = `mem_total` - (`mem_free` + `mem_buffers` + `mem_cached`)

