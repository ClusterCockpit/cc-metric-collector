<!--
---
title: Load average metric collector
description: Collect metrics from `/proc/loadavg`
categories: [cc-metric-collector]
tags: ['Admin']
weight: 2
hugo_path: docs/reference/cc-metric-collector/collectors/loadavg.md
---
-->


## `loadavg` collector

```json
  "loadavg": {
    "exclude_metrics": [
      "proc_run"
    ]
  }
```

The `loadavg` collector reads data from `/proc/loadavg` and outputs a handful **node** metrics. If a metric is not required, it can be excluded from forwarding it to the sink.

Metrics:
* `load_one`
* `load_five`
* `load_fifteen`
* `proc_run`
* `proc_total`
