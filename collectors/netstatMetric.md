## `netstat` collector

```json
  "netstat": {
    "include_devices": [
      "eth0",
      "eno1"
    ],
    "send_abs_values": true,
    "send_derived_values": true,
    "interface_aliases": {
      "eno1": ["eno1np0", "eno1_alt"],
      "eth0": ["eth0_alias"]
    },
    "exclude_metrics": [
      "net_pkts_in"
    ],
    "only_metrics": [
      "net_bytes_in_bw"
    ]
  }
```

The `netstat` collector reads data from `/proc/net/dev` and outputs a handful **node** metrics. With the `include_devices` list you can specify which network interfaces should be measured. Optionally, you can define an `interface_aliases` mapping. For each canonical device (as listed in include_devices), you may provide an array of aliases that may be reported by the system. When an alias is detected, it is mapped to the canonical name, while the output tag `stype-id` always shows the actual system-reported name.

Both filtering mechanisms are supported:
- `exclude_metrics`: Excludes the specified metrics.
- `only_metrics`: If provided, only the listed metrics are collected. This takes precedence over `exclude_metrics`.

**Absolute Metrics:**
- `net_bytes_in` (unit: `bytes`)
- `net_bytes_out` (unit: `bytes`)
- `net_pkts_in` (unit: `packets`)
- `net_pkts_out` (unit: `packets`)

**Derived Metrics:**
- `net_bytes_in_bw` (unit: `bytes/sec`)
- `net_bytes_out_bw` (unit: `bytes/sec`)
- `net_pkts_in_bw` (unit: `packets/sec`)
- `net_pkts_out_bw` (unit: `packets/sec`)

The device name is added as tag `stype=network,stype-id=<device>`.
