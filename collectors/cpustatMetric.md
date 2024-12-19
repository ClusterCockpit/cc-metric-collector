
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

* `cpu_user` with `unit=Percent`
* `cpu_nice` with `unit=Percent`
* `cpu_system` with `unit=Percent`
* `cpu_idle` with `unit=Percent`
* `cpu_iowait` with `unit=Percent`
* `cpu_irq` with `unit=Percent`
* `cpu_softirq` with `unit=Percent`
* `cpu_steal` with `unit=Percent`
* `cpu_guest` with `unit=Percent`
* `cpu_guest_nice` with `unit=Percent`
* `cpu_used` = `cpu_* - cpu_idle` with `unit=Percent`
* `num_cpus`