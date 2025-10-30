<!--
---
title: GPFS collector
description: Collect infos about GPFS filesystems
categories: [cc-metric-collector]
tags: ['Admin']
weight: 2
hugo_path: docs/reference/cc-metric-collector/collectors/gpfs.md
---
-->

## `gpfs` collector

```json
  "gpfs": {
    "mmpmon_path": "/path/to/mmpmon",
    "use_sudo": "true",
    "exclude_filesystem": [
      "fs1"
    ],
    "exclude_metrics": [
      "gpfs_bytes_written"
    ],
    "send_abs_values": true,
    "send_diff_values": true,
    "send_derived_values": true,
    "send_total_values": true,
    "send_bandwidths": true
  }
```

The `gpfs` collector uses the `mmpmon` command to read performance metrics for
GPFS / IBM Spectrum Scale filesystems.

The reported filesystems can be filtered with the `exclude_filesystem` option
in the configuration.
Individual metrics can be disabled for reporting using option `exclude_metrics`.

The path to the `mmpmon` command can be configured with the `mmpmon_path` option
in the configuration. If nothing is set, the collector searches in `$PATH` for `mmpmon`.

If cc-metric-collector is run as non-root, password-less `sudo` can be enabled with `use_sudo`. 
Because `mmpmon` is by default only executable as root, the Go procedure to
search for it in `$PATH` will fail. If you use `sudo`, you must specify the
complete path for `mmpmon` using the parameter `mmpmon_path`.


Metrics:
* `gpfs_bytes_read` (if `send_abs_values == true`)
* `gpfs_bytes_written` (if `send_abs_values == true`)
* `gpfs_num_opens` (if `send_abs_values == true`)
* `gpfs_num_closes` (if `send_abs_values == true`)
* `gpfs_num_reads` (if `send_abs_values == true`)
* `gpfs_num_writes` (if `send_abs_values == true`)
* `gpfs_num_readdirs` (if `send_abs_values == true`)
* `gpfs_num_inode_updates` (if `send_abs_values == true`)
* `gpfs_bytes_read_diff` (if `send_diff_values == true`)
* `gpfs_bytes_written_diff` (if `send_diff_values == true`)
* `gpfs_num_opens_diff` (if `send_diff_values == true`)
* `gpfs_num_closes_diff` (if `send_diff_values == true`)
* `gpfs_num_reads_diff` (if `send_diff_values == true`)
* `gpfs_num_writes_diff` (if `send_diff_values == true`)
* `gpfs_num_readdirs_diff` (if `send_diff_values == true`)
* `gpfs_num_inode_updates_diff` (if `send_diff_values == true`)
* `gpfs_bw_read` (if `send_derived_values == true` or `send_bandwidths == true`)
* `gpfs_bw_write` (if `send_derived_values == true` or `send_bandwidths == true`)
* `gpfs_opens_rate` (if `send_derived_values == true`)
* `gpfs_closes_rate` (if `send_derived_values == true`)
* `gpfs_reads_rate` (if `send_derived_values == true`)
* `gpfs_writes_rate` (if `send_derived_values == true`)
* `gpfs_readdirs_rate` (if `send_derived_values == true`)
* `gpfs_inode_updates_rate` (if `send_derived_values == true`)
* `gpfs_bytes_total = gpfs_bytes_read + gpfs_bytes_written` (if `send_total_values == true` and `send_abs_values == true`)
* `gpfs_bytes_total_diff` (if `send_total_values == true` and `send_diff_values == true`)
* `gpfs_bw_total` ((if `send_total_values == true` and `send_derived_values == true`) or `send_bandwidths == true`)
* `gpfs_iops = gpfs_num_reads + gpfs_num_writes` (if `send_total_values == true` and `send_abs_values == true`)
* `gpfs_iops_diff` (if `send_total_values == true` and `send_diff_values == true`)
* `gpfs_iops_rate` (if `send_total_values == true` and `send_derived_values == true`)
* `gpfs_metaops = gpfs_num_inode_updates + gpfs_num_closes + gpfs_num_opens + gpfs_num_readdirs` (if `send_total_values == true` and `send_abs_values == true`)
* `gpfs_metaops_diff` (if `send_total_values == true` and `send_diff_values == true`)
* `gpfs_metaops_rate` (if `send_total_values == true` and `send_derived_values == true`)

The collector adds a `filesystem` tag to all metrics
