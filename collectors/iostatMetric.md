
## `iostat` collector

```json
  "iostat": {
    "exclude_metrics": [
      "read_ms"
    ],
  }
```

The `iostat` collector reads data from `/proc/diskstats` and outputs a handful **node** metrics. If a metric is not required, it can be excluded from forwarding it to the sink.

Metrics:
* `io_reads`
* `io_reads_merged`
* `io_read_sectors`
* `io_read_ms`
* `io_writes`
* `io_writes_merged`
* `io_writes_sectors`
* `io_writes_ms`
* `io_ioops`
* `io_ioops_ms`
* `io_ioops_weighted_ms`
* `io_discards`
* `io_discards_merged`
* `io_discards_sectors`
* `io_discards_ms`
* `io_flushes`
* `io_flushes_ms`

The device name is added as tag `device`. For more details, see https://www.kernel.org/doc/html/latest/admin-guide/iostats.html

