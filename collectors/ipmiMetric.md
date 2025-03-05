## `ipmistat` collector

```json
  "ipmistat": {
    "ipmitool_path": "/path/to/ipmitool",
    "ipmisensors_path": "/path/to/ipmi-sensors",
    "exclude_metrics": [
      "cpu0_temp"
    ],
    "only_metrics": [
      "sys_power",
      "inlet_air_temp"
    ]
  }
```

The `ipmistat` collector retrieves sensor data via IPMI. By default, if `ipmitool` is found in the system’s `PATH` and executes successfully with "-V", it is used to run `ipmitool sensor`. Only if `ipmitool` is not found or fails, the collector falls back to `ipmi-sensors` (executed with `--comma-separated-output --sdr-cache-recreate`).

Both filtering mechanisms are supported:
- `exclude_metrics`: Excludes the specified metrics.
- `only_metrics`: If provided, only the listed metrics are collected. This takes precedence over `exclude_metrics`.

The metrics depend on the output of the underlying tools but contain temperature, power and energy metrics.
