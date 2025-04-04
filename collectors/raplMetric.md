## `rapl` collector

This collector reads running average power limit (RAPL) monitoring attributes to compute average power consumption metrics. See <https://www.kernel.org/doc/html/latest/power/powercap/powercap.html>.

```json
  "rapl": {
    "exclude_device_by_id": ["0:1", "0:2"],
    "exclude_device_by_name": ["psys"],
    "skip_energy_reading": false,
    "skip_limits_reading": false,
    "only_enabled_limits": true
  }
```

Metrics:
* `rapl_<domain>_average_power`: average power consumption in Watt. The average is computed over the entire runtime from the last measurement to the current measurement
* `rapl_<domain>_energy`: Difference from the last measurement
* `rapl_<domain>_limit_short_term`: Short term powercap setting for the domain
* `rapl_<domain>_limit_long_term`: Long term powercap setting for the domain

Only the `rapl_<domain>_average_power` and `rapl_<domain>_energy` metrics require root-permissions. The limits can be read as user. Some domains have limits available but they are not enabled. By default, only enabled domain limits are collected.

Energy and power measurments can also be done with the Likwid metric collector.