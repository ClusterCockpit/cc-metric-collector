<!--
---
title: IPMI Metric collector
description: Collect metrics using ipmitool or ipmi-sensors
categories: [cc-metric-collector]
tags: ['Admin']
weight: 2
hugo_path: docs/reference/cc-metric-collector/collectors/ipmi.md
---
-->

## `ipmistat` collector

```json
  "ipmistat": {
    "ipmitool_path": "/path/to/ipmitool",
    "ipmisensors_path": "/path/to/ipmi-sensors",
  }
```

The `ipmistat` collector reads data from `ipmitool` (`ipmitool sensor`) or `ipmi-sensors` (`ipmi-sensors --sdr-cache-recreate --comma-separated-output`).

The metrics depend on the output of the underlying tools but contain temperature, power and energy metrics.
