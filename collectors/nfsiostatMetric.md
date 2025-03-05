## `nfsiostat` collector

```json
  "nfsiostat": {
    "exclude_metrics": [
      "oread", "pageread"
    ],
    "only_metrics": [
      "nread", "nwrite", "nfsread", "nfswrite"
    ],
    "exclude_filesystem": [
      "/mnt"
    ],
    "use_server_as_stype": false,
    "send_abs_values": false,
    "send_derived_values": true
  }
```

The `nfsiostat` collector reads data from `/proc/self/mountstats` and outputs a handful **node**s metrics for each NFS filesystem.
Metrics are output with the prefix `nfsio_` and the base metric name (e.g. `nread`, `nwrite`, etc.). Filtering applies to the base metric name (without the `nfsio_` prefix).

Both filtering mechanisms are supported:
- `exclude_metrics`: Excludes the specified metrics.
- `only_metrics`: If provided, only the listed metrics are collected. This takes precedence over `exclude_metrics`.

Metrics are categorized as follows:

**Absolute Metrics:**
- `nfsio_nread`: Bytes transferred by normal read() calls
- `nfsio_nwrite`: Bytes transferred by normal write() calls
- `nfsio_oread`: Bytes transferred by read() calls with O_DIRECT
- `nfsio_owrite`: Bytes transferred by write() calls with O_DIRECT
- `nfsio_pageread`: Pages transferred by read() calls
- `nfsio_pagewrite`: Pages transferred by write() calls
- `nfsio_nfsread`: Bytes transferred for reading from the server
- `nfsio_nfswrite`: Bytes transferred for writing to the server

**Derived Metrics:**
For each absolute metric, if `send_derived_values` is enabled, an additional metric is sent with the `_bw` suffix, representing the rate:
- For byte metrics: `unit=bytes/sec`
- For page metrics: `unit=4K_pages/s`

The `nfsiostat` collector adds the mountpoint to the tags as `stype=filesystem,stype-id=<mountpoint>`. If the server address should be used instead of the mountpoint, use the `use_server_as_stype` config setting.
