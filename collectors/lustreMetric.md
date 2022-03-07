
## `lustrestat` collector

```json
  "lustrestat": {
    "procfiles" : [
      "/proc/fs/lustre/llite/lnec-XXXXXX/stats"
    ],
    "exclude_metrics": [
      "setattr",
      "getattr"
    ],
    "send_abs_values" : true,
    "send_derived_values" : true
  }
```

The `lustrestat` collector reads from the procfs stat files for Lustre like `/proc/fs/lustre/llite/lnec-XXXXXX/stats`.

Metrics:
* `lustre_read_bytes`
* `lustre_read_requests`
* `lustre_write_bytes`
* `lustre_write_requests`
* `lustre_open`
* `lustre_close`
* `lustre_getattr`
* `lustre_setattr`
* `lustre_statfs`
* `lustre_inode_permission`
* `lustre_read_bytes_bw` (if `send_derived_values == true`)
* `lustre_write_bytes_bw` (if `send_derived_values == true`)

This collector adds an `device` tag.