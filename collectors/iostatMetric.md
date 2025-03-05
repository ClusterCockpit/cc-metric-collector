## `iostat` collector

```json
  "iostat": {
    "exclude_metrics": [
      "io_read_ms"
    ],
    "exclude_devices": [
      "nvme0n1p1",
      "nvme0n1p2",
      "md127"
    ],
    "only_metrics": [],
    "send_abs_values": true,
    "send_diff_values": true,
    "send_derived_values": true
  }
```

The `iostat` collector reads data from `/proc/diskstats` and outputs a handful **node** metrics. If a metric is not required, it can be excluded from forwarding it to the sink.

Both filtering mechanisms are supported:
- `exclude_metrics`: Excludes the specified metrics.
- `only_metrics`: If provided, only the listed metrics are collected. This takes precedence over `exclude_metrics`.

**Absolute Metrics:**
- `io_reads`
- `io_reads_merged`
- `io_read_sectors`
- `io_read_ms`
- `io_writes`
- `io_writes_merged`
- `io_writes_sectors`
- `io_writes_ms`
- `io_ioops`
- `io_ioops_ms`
- `io_ioops_weighted_ms`
- `io_discards`
- `io_discards_merged`
- `io_discards_sectors`
- `io_discards_ms`
- `io_flushes`
- `io_flushes_ms`

**Diff Metrics:**
For each metric, if `send_diff_values` is enabled, the collector computes the difference (current value minus previous value) and sends it with the suffix `_diff`.

**Derived Metrics:**
For each metric, if `send_derived_values` is enabled, the collector computes the derived rate (difference divided by the time interval) and sends it with the suffix `_rate`.

The device name is added as tag `device`. For more details, see https://www.kernel.org/doc/html/latest/admin-guide/iostats.html
