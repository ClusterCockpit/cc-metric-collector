
## `netstat` collector

```json
  "netstat": {
    "include_devices": [
      "eth0"
    ],
    "send_abs_values" : true,
    "send_derived_values" : true
  }
```

The `netstat` collector reads data from `/proc/net/dev` and outputs a handful **node** metrics. With the `include_devices` list you can specify which network devices should be measured. **Note**: Most other collectors use an _exclude_ list instead of an include list.

Metrics:
* `net_bytes_in` (`unit=bytes`)
* `net_bytes_out` (`unit=bytes`)
* `net_pkts_in` (`unit=packets`)
* `net_pkts_out` (`unit=packets`)
* `net_bytes_in_bw` (`unit=bytes/sec` if `send_derived_values == true`)
* `net_bytes_out_bw` (`unit=bytes/sec` if `send_derived_values == true`)
* `net_pkts_in_bw` (`unit=packets/sec` if `send_derived_values == true`)
* `net_pkts_out_bw` (`unit=packets/sec` if `send_derived_values == true`)

The device name is added as tag `stype=network,stype-id=<device>`.

