<!--
---
title: smartmon metric collector
description: Collect S.M.A.R.T data from NVMEs
categories: [cc-metric-collector]
tags: ['Admin']
weight: 2
hugo_path: docs/reference/cc-metric-collector/collectors/smartmonMetric.md
---
-->

## `smartmon` collector

```json
  "smartmon": {
    "use_sudo" : true,
    "exclude_devices": [
      "/dev/sda"
    ],
    "excludeMetrics": [
      "smartmon_warn_temp_time",
      "smartmon_crit_comp_time"
    ]
    "devices": [
      "name": "/dev/nvme0"
      "type": "nvme"
    ]
  }
```

The `smartmon` collector retrieves S.M.A.R.T data from NVMEs via command `smartctl`.

Available NVMEs can be either automatically detected by a device scan or manually added with the "devices" config option.

Metrics:

* `smartmon_temp`: Temperature of the device (`unit=degC`)
* `smartmon_avail_spare`: Amount of spare left (`unit=percent`)
* `smartmon_percent_used`: Percentage of the device is used (`unit=percent`)
* `smartmon_data_units_read`: Read data units
* `smartmon_data_units_write`: Written data units
* `smartmon_host_reads`: Read operations
* `smartmon_host_writes`: Write operations
* `smartmon_power_cycles`: Number of power cycles
* `smartmon_power_on`: Seconds the device is powered on (`unit=seconds`)
* `smartmon_unsafe_shutdowns`: Count of unsafe shutdowns
* `smartmon_media_errors`: Media errors of the device
* `smartmon_errlog_entries`: Error log entries
* `smartmon_warn_temp_time`: Time above the warning temperature threshold
* `smartmon_crit_comp_time`: Time above the critical composite temperature threshold
