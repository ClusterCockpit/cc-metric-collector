
## `netstat` collector

```json
  "netstat": {
    "exclude_devices": [
      "lo"
    ]
  }
```

The `netstat` collector reads data from `/proc/net/dev` and outputs a handful **node** metrics. If a device is not required, it can be excluded from forwarding it to the sink. Commonly the `lo` device should be excluded.

Metrics:
* `bytes_in`
* `bytes_out`
* `pkts_in`
* `pkts_out`

The device name is added as tag `device`.

