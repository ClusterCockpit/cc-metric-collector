## `nfs3stat` collector

```json
  "nfs3stat": {
    "nfsstat": "/path/to/nfsstat",
    "exclude_metrics": [
      "total"
    ],
    "only_metrics": [
      "read_diff"
    ],
    "send_abs_values": true,
    "send_diff_values": true,
    "send_derived_values": true
  }
```

The `nfs3stat` collector reads data from `nfsstat` command and outputs a handful **node** metrics.

Both filtering mechanisms are supported:
- `exclude_metrics`: Excludes the specified metrics. Uses base metric names without the nfs3_ prefix.
- `only_metrics`: If provided, only the listed metrics are collected. This takes precedence over `exclude_metrics`.

**Absolute Metrics:**
- `nfs3_total`
- `nfs3_null`
- `nfs3_getattr`
- `nfs3_setattr`
- `nfs3_lookup`
- `nfs3_access`
- `nfs3_readlink`
- `nfs3_read`
- `nfs3_write`
- `nfs3_create`
- `nfs3_mkdir`
- `nfs3_symlink`
- `nfs3_remove`
- `nfs3_rmdir`
- `nfs3_rename`
- `nfs3_link`
- `nfs3_readdir`
- `nfs3_readdirplus`
- `nfs3_fsstat`
- `nfs3_fsinfo`
- `nfs3_pathconf`
- `nfs3_commit`

**Diff Metrics:**
For each metric, if `send_diff_values` is enabled, the collector computes the difference (current value minus previous value) and sends it with the suffix `_diff`.

**Derived Metrics:**
For each metric, if `send_derived_values` is enabled, the collector computes the rate (difference divided by the time interval) and sends it with the suffix `_rate`.
