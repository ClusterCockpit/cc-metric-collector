
## `netstat` collector

```json
  "netstat": {
    "include_devices": [
      "eth0"
    ]
  }
```

The `netstat` collector reads data from `/proc/net/dev` and outputs a handful **node** metrics. With the `include_devices` list you can specify which network devices should be measured. **Note**: Most other collectors use an _exclude_ list instead of an include list.

Metrics:
* `net_bytes_in` (`unit=bytes/sec`)
* `net_bytes_out` (`unit=bytes/sec`)
* `net_pkts_in` (`unit=packets/sec`)
* `net_pkts_out` (`unit=packets/sec`)

The device name is added as tag `device`.

