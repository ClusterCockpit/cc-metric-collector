<!--
---
title: Memory statistics metric collector
description: Collect metrics from `/proc/meminfo`
categories: [cc-metric-collector]
tags: ['Admin']
weight: 2
hugo_path: docs/reference/cc-metric-collector/collectors/memstat.md
---
-->


## `memstat` collector

```json
  "memstat": {
    "exclude_metrics": [
      "mem_used"
    ]
  }
```

The `memstat` collector reads data from `/proc/meminfo` and outputs a handful **node** metrics. If a metric is not required, it can be excluded from forwarding it to the sink.


Metrics:
* `mem_total`
* `mem_sreclaimable`
* `mem_slab`
* `mem_free`
* `mem_buffers`
* `mem_cached`
* `mem_available`
* `mem_shared`
* `swap_total`
* `swap_free`
* `mem_used` = `mem_total` - (`mem_free` + `mem_buffers` + `mem_cached`)

