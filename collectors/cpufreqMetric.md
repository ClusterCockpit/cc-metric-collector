## `cpufreq_cpuinfo` collector
```json
  "cpufreq": {
    "exclude_metrics": []
  }
```

The `cpufreq` collector reads the clock frequency from `/sys/devices/system/cpu/cpu*/cpufreq` and outputs a handful **cpu** metrics.

Metrics:
* `cpufreq`