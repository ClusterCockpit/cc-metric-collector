
## `diskstat` collector

```json
  "diskstat": {
    "exclude_metrics": [
      "read_ms"
    ],
  }
```

The `netstat` collector reads data from `/proc/net/dev` and outputs a handful **node** metrics. If a metric is not required, it can be excluded from forwarding it to the sink.

Metrics:
* `reads`
* `reads_merged`
* `read_sectors`
* `read_ms`
* `writes`
* `writes_merged`
* `writes_sectors`
* `writes_ms`
* `ioops`
* `ioops_ms`
* `ioops_weighted_ms`
* `discards`
* `discards_merged`
* `discards_sectors`
* `discards_ms`
* `flushes`
* `flushes_ms`

The device name is added as tag `device`.

