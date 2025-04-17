<!--
---
title: NFS network filesystem metrics from procfs
description: Collect NFS network filesystem metrics for mounts from `/proc/self/mountstats`
categories: [cc-metric-collector]
tags: ['Admin']
weight: 2
hugo_path: docs/reference/cc-metric-collector/collectors/nfsio.md
---
-->

## `nfsiostat` collector

```json
  "nfsiostat": {
    "exclude_metrics": [
      "oread", "pageread"
    ],
    "exclude_filesystems": [
      "/mnt"
    ],
    "use_server_as_stype": false,
    "send_abs_values": false,
    "send_derived_values": true
  }
```

The `nfsiostat` collector reads data from `/proc/self/mountstats` and outputs a handful **node** metrics for each NFS filesystem. If a metric or filesystem is not required, it can be excluded from forwarding it to the sink. **Note:** When excluding metrics, you must provide the base metric name (e.g. pageread) without the nfsio_ prefix. This exclusion applies to both absolute and derived values.

Metrics:
* `nfsio_nread`: Bytes transferred by normal `read()` calls
* `nfsio_nwrite`: Bytes transferred by normal `write()` calls
* `nfsio_oread`: Bytes transferred by `read()` calls with `O_DIRECT`
* `nfsio_owrite`: Bytes transferred by `write()` calls with `O_DIRECT`
* `nfsio_pageread`: Pages transferred by `read()` calls
* `nfsio_pagewrite`: Pages transferred by `write()` calls
* `nfsio_nfsread`: Bytes transferred for reading from the server
* `nfsio_nfswrite`: Pages transferred by writing to the server

For each of these, if derived values are enabled, an additional metric is sent with the `_bw` suffix, which represents the rate:

  * For normal byte metrics: `unit=bytes/sec`
  * For page metrics: `unit=4K_pages/s`

The `nfsiostat` collector adds the mountpoint to the tags as `stype=filesystem,stype-id=<mountpoint>`. If the server address should be used instead of the mountpoint, use the `use_server_as_stype` config setting.
