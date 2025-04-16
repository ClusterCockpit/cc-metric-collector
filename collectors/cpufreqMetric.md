<!--
---
title: CPU frequency metric collector through sysfs
description: Collect the CPU frequency metrics from `/sys/.../cpu/.../cpufreq`
categories: [cc-metric-collector]
tags: ['Admin']
weight: 2
hugo_path: docs/reference/cc-metric-collector/collectors/cpufreq.md
---
-->

## `cpufreq_cpuinfo` collector

```json
  "cpufreq": {
    "exclude_metrics": []
  }
```

The `cpufreq` collector reads the clock frequency from `/sys/devices/system/cpu/cpu*/cpufreq` and outputs a handful **hwthread** metrics.

Metrics:

* `cpufreq`
