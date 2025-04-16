<!--
---
title: TopProcs collector
description: Collect infos about most CPU-consuming processes
categories: [cc-metric-collector]
tags: ['Admin']
weight: 2
hugo_path: docs/reference/cc-metric-collector/collectors/topprocs.md
---
-->



## `topprocs` collector

```json
  "topprocs": {
    "num_procs": 5
  }
```

The `topprocs` collector reads the TopX processes (sorted by CPU utilization, `ps -Ao comm --sort=-pcpu`). 

In contrast to most other collectors, the metric value is a `string`.



