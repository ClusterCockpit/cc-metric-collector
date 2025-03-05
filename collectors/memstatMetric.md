## `memstat` collector

```json
  "memstat": {
    "exclude_metrics": [
      "mem_buffera"
    ],
    "only_metrics": [
      "mem_total",
      "mem_free",
      "mem_cached",
      "mem_used"
    ]
  }
```

The `memstat` collector reads data from `/proc/meminfo` and outputs a handful **node** metrics. If a metric is not required, it can be excluded from forwarding it to the sink.

Both filtering mechanisms are supported:
- `exclude_metrics`: Excludes the specified metrics.
- `only_metrics`: If provided, only the listed metrics are collected. This takes precedence over `exclude_metrics`.


Metrics:
- `mem_total`
- `mem_sreclaimable`
- `mem_slab`
- `mem_free`
- `mem_buffers`
- `mem_cached`
- `mem_available`
- `mem_shared`
- `swap_total`
- `swap_free`
- `mem_used` = `mem_total` - (`mem_free` + `mem_buffers` + `mem_cached`)
