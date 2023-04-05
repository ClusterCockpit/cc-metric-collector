
## `cpustat` collector

```json
  "cpustat": {
    "exclude_metrics": [
      "cpu_idle"
    ]
  }
```

The `cpustat` collector reads data from `/proc/stat` and outputs a handful **node** and **hwthread** metrics. If a metric is not required, it can be excluded from forwarding it to the sink.

Metrics:

* `cpu_user`
* `cpu_nice`
* `cpu_system`
* `cpu_idle`
* `cpu_iowait`
* `cpu_irq`
* `cpu_softirq`
* `cpu_steal`
* `cpu_guest`
* `cpu_guest_nice`
* `cpu_used` = `cpu_* - cpu_idle`