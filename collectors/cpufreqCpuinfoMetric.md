<!--
---
title: CPU frequency metric collector through cpuinfo
description: Collect the CPU frequency from `/proc/cpuinfo`
categories: [cc-metric-collector]
tags: ['Admin']
weight: 2
hugo_path: docs/reference/cc-metric-collector/collectors/cpufreq_cpuinfo.md
---
-->

## `cpufreq_cpuinfo` collector

```json
  "cpufreq_cpuinfo": {}
```

The `cpufreq_cpuinfo` collector reads the clock frequency from `/proc/cpuinfo` and outputs a handful **hwthread** metrics.

Metrics:

* `cpufreq`
