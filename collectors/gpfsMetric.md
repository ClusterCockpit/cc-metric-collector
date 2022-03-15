## `gpfs` collector

```json
  "ibstat": {
    "mmpmon_path": "/path/to/mmpmon",
    "exclude_filesystem": [
      "fs1"
    ],
    "send_bandwidths" : true
  }
```

The `gpfs` collector uses the `mmpmon` command to read performance metrics for
GPFS / IBM Spectrum Scale filesystems.

The reported filesystems can be filtered with the `exclude_filesystem` option
in the configuration.

The path to the `mmpmon` command can be configured with the `mmpmon_path` option
in the configuration. If nothing is set, the collector searches in `$PATH` for `mmpmon`.


Metrics:
* `gpfs_bytes_read`
* `gpfs_bytes_written`
* `gpfs_num_opens`
* `gpfs_num_closes`
* `gpfs_num_reads`
* `gpfs_num_readdirs`
* `gpfs_num_inode_updates`
* `gpfs_bw_read` (if `send_bandwidths == true`)
* `gpfs_bw_write` (if `send_bandwidths == true`)

The collector adds a `filesystem` tag to all metrics
