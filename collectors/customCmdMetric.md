## `customcmd` collector

```json
  "customcmd": {
    "exclude_metrics": [
      "mymetric"
    ],
    "only_metrics": [
      "cpu_usage",
      "cpu_usage_rate",
      "mem_usage",
      "mem_usage_rate"
    ],
    "send_abs_values": true,
    "send_diff_values": true,
    "send_derived_values": true,
    "files": [
      "/var/run/myapp.metrics"
    ],
    "commands": [
      "/usr/local/bin/getmetrics.pl"
    ]
  }
```

The `customcmd` collector reads data from specified files and executed commands.
Both the output of commands and the content of files must follow the [InfluxDB line protocol](https://docs.influxdata.com/influxdb/cloud/reference/syntax/line-protocol/).

**Expected format example:**

```
cpu_usage,host=myhost,type=hwthread,type-id=0,unit=MByte value=42.0 1670000000000000000
mem_usage,host=myhost,type=node,unit=MByte value=1024 1670000000000000000
```

The following tags are commonly used:
- **type:** Indicates the metric scope, e.g. "node", "socket" or "hwthread".
- **type-id:** The identifier for the type (e.g. the specific hardware thread or socket).
- **unit:** The unit of the metric (e.g. "MByte").

For each metric parsed from the output:
- If `send_abs_values` is enabled, the **absolute (raw) metric** is forwarded.
- If `send_diff_values` is enabled and a previous value exists, the collector computes the **difference** (current value minus previous value) and forwards it as a new metric with the suffix `_diff`.
- If `send_derived_values` is enabled and a previous value exists, the collector computes the **derived rate** (difference divided by the time interval) and forwards it as a new metric with the suffix `_rate`.
  Additionally, if the original metric includes a unit (in its meta data or tags), the derived metric's unit is set to that unit with "/s" appended.

Both filtering mechanisms are supported:
- `exclude_metrics`: Excludes the specified metrics.
- `only_metrics`: If provided, only the listed metrics are collected. This takes precedence over `exclude_metrics`.
