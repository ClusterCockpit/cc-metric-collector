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
    "exclude_devices": [
      "slurm-tmpfs"
    ],
    "exclude_mountpoints": [
      "/tmp"
    ],
    "mountpoint_as_stype": true,
    "use_include_config": false,
    "include_devices": [
      "/dev/sda3"
    ],
    "include_mountpoints" : [
      "/home"
    ]
  }
```

The `diskstat` collector reads data from `/proc/self/mounts` and outputs a handful **node** metrics with `stype=filesystem,stype-id=<mountdevice>`. If a metric is not required, it can be excluded from forwarding it to the sink.

For sending the `mountpoint` instead of the `mountdevice` in the `stype-id`, use `mountpoint_as_stype`.

There are two ways to specify for which devices or mountpoints the collector generates metrics. It's "either ...or".
  - Excluding devices and mount points using `exclude_devices` and `exclude_mountpoints`. All devices (*) will be read that are not explicitly excluded
  - Include devices and mount points by setting `use_include_config:true` and using `include_devices` and `include_mountpoints`.

(*) File systems where the mount device (first column in `/proc/self/mounts`) contains `loop` are always excluded. Filesystems where the mount point (second column in `/proc/self/mounts`) contains `boot` are also always excluded.

Metrics per filesystem (with `stype=filesystem` tag and `stype-id` based on the configuration):
* `disk_total` (unit `GBytes`)
* `disk_free` (unit `GBytes`)

Global metrics (with `stype=filesystem` tag and `stype-id` pointing to the max. used filesystem device or mount point based on the configuration):
* `part_max_used` (unit `percent`)


