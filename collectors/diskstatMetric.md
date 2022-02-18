
## `diskstat` collector

```json
  "diskstat": {
    "exclude_metrics": [
      "disk_total"
    ],
  }
```

The `diskstat` collector reads data from `/proc/self/mounts` and outputs a handful **node** metrics. If a metric is not required, it can be excluded from forwarding it to the sink.

Metrics per device (with `device` tag):
* `disk_total` (unit `GBytes`)
* `disk_free` (unit `GBytes`)

Global metrics:
* `part_max_used` (unit `percent`)


