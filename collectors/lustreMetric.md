
## `lustrestat` collector

```json
  "lustrestat": {
    "procfiles" : [
      "/proc/fs/lustre/llite/lnec-XXXXXX/stats"
    ],
    "exclude_metrics": [
      "setattr",
      "getattr"
    ]
  }
```

The `lustrestat` collector reads from the procfs stat files for Lustre like `/proc/fs/lustre/llite/lnec-XXXXXX/stats`.

Metrics:
* `read_bytes`
* `read_requests`
* `write_bytes`
* `write_requests`
* `open`
* `close`
* `getattr`
* `setattr`
* `statfs`
* `inode_permission`

