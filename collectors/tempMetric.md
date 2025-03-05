## tempstat collector

```json{
  "tempstat": {
    "tag_override": {
        "<device identifier>": {
            "type": "socket",
            "type-id": "0"
        }
    },
    "exclude_metrics": [
      "metric1",
      "metric2"
    ],
    "only_metrics": [
      "temp_core_0",
      "temp_core_1"
    ],
    "report_max_temperature": true,
    "report_critical_temperature": true
  }
```

The `tempstat` collector reads the data from `/sys/class/hwmon/<device>/tempX_{input,label}`.

Both filtering mechanisms are supported:
- `exclude_metrics`: Excludes the specified metrics.
- `only_metrics`: If provided, only the listed metrics are collected. This takes precedence over `exclude_metrics`.

Metrics:
- `temp_*`: The metric name is taken from the label files.

Optional additional metrics:
- **Max Temperature:** If `report_max_temperature` is enabled, the collector also reads the maximum temperature from the corresponding `_max` file. The metric name is derived by replacing "temp" with "max_temp" in the sensor's metric name.
- **Critical Temperature:** If `report_critical_temperature` is enabled, the collector also reads the critical temperature from the corresponding `_crit` file. The metric name is derived by replacing "temp" with "crit_temp" in the sensor's metric name.
