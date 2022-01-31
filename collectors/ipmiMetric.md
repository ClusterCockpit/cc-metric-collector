
## `ipmistat` collector

```json
  "ipmistat": {
    "ipmitool_path": "/path/to/ipmitool",
    "ipmisensors_path": "/path/to/ipmi-sensors",
  }
```

The `ipmistat` collector reads data from `ipmitool` (`ipmitool sensor`) or `ipmi-sensors` (`ipmi-sensors --sdr-cache-recreate --comma-separated-output`). 

The metrics depend on the output of the underlying tools but contain temperature, power and energy metrics.



