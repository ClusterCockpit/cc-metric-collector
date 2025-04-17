<!--
---
title: Disk usage statistics metric collector
description: Collect metrics for various filesystems from `/proc/self/mounts`
categories: [cc-metric-collector]
tags: ['Admin']
weight: 2
hugo_path: docs/reference/cc-metric-collector/collectors/diskstat.md
---
-->

## `diskstat` collector

```json
  "diskstat": {
    "exclude_metrics": [
      "disk_total"
    ],
    "exclude_mounts": [
      "slurm-tmpfs"
    ]
  }
```

The `diskstat` collector reads data from `/proc/self/mounts` and outputs a handful **node** metrics. If a metric is not required, it can be excluded from forwarding it to the sink. Additionally, any mount point containing one of the strings specified in `exclude_mounts` will be skipped during metric collection.

Metrics per device (with `device` tag):
* `disk_total` (unit `GBytes`)
* `disk_free` (unit `GBytes`)

Global metrics:
* `part_max_used` (unit `percent`)


