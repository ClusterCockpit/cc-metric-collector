## `cpustat` collector

```json
  "cpustat": {
    "exclude_metrics": [
      "cpu_idle"
    ],
    "only_metrics": [
      "cpu_user"
    ]
  }
```

The `cpustat` collector reads data from `/proc/stat` and outputs a handful **node** and **hwthread** metrics.

Both filtering mechanisms are supported:
- `exclude_metrics`: Excludes the specified metrics.
- `only_metrics`: If provided, only the listed metrics are collected. This takes precedence over `exclude_metrics`.

Metrics:

- `cpu_user` (unit: `Percent`)
- `cpu_nice` (unit: `Percent`)
- `cpu_system` (unit: `Percent`)
- `cpu_idle` (unit: `Percent`)
- `cpu_iowait` (unit: `Percent`)
- `cpu_irq` (unit: `Percent`)
- `cpu_softirq` (unit: `Percent`)
- `cpu_steal` (unit: `Percent`)
- `cpu_guest` (unit: `Percent`)
- `cpu_guest_nice` (unit: `Percent`)
- `cpu_used` = `cpu_* - cpu_idle` (unit: `Percent`)
- `num_cpus`
