<!--
---
title: SchedStat Metric collector
description: Collect metrics from `/proc/schedstat`
categories: [cc-metric-collector]
tags: ['Admin']
weight: 2
hugo_path: docs/reference/cc-metric-collector/collectors/schedstat.md
---
-->

## `schedstat` collector
```json
  "schedstat": {
  }
```

The `schedstat` collector reads data from /proc/schedstat and calculates a load value, separated by hwthread. This might be useful to detect bad cpu pinning on shared nodes etc. 

Metric:
* `cpu_load_core`
