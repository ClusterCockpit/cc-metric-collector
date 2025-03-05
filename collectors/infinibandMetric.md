## `ibstat` collector

```json
  "ibstat": {
    "exclude_devices": [
      "mlx4"
    ],
    "exclude_metrics": [
      "ib_total"
    ],
    "only_metrics": [
      "ib_revc_bw"
    ],
    "send_abs_values": true,
    "send_derived_values": true,
    "send_total_values": true
  }
```

The ibstat collector includes all InfiniBand devices found under `/sys/class/infiniband/` for which a LID file (`/sys/class/infiniband/<dev>/ports/<port>/lid`) is present.
Devices can be filtered with the `exclude_devices` option.

For each found LID the collector reads data through the sysfs files below `/sys/class/infiniband/<device>`. (See: <https://www.kernel.org/doc/Documentation/ABI/stable/sysfs-class-infiniband>)

Both filtering mechanisms are supported:
- `exclude_metrics`: Excludes the specified metrics.
- `only_metrics`: If provided, only the listed metrics are collected. This takes precedence over `exclude_metrics`.

**Absolute Metrics:**
- `ib_recv` (unit: `bytes`)
- `ib_xmit` (unit: `bytes`)
- `ib_recv_pkts` (unit: `packets`)
- `ib_xmit_pkts` (unit: `packets`)

**Derived Metrics:**
- `ib_recvi_bw` (unit: `bytes/s`)
- `ib_xmit_bw` (unit: `bytes/s`)
- `ib_recv_pkts_bw` (unit: `packets/s`)
- `ib_xmit_pkts_bw` (unit: `packets/s`)

**Global metrics** (if `send_total_values` is enabled):
- `ib_total` = ib_recv + ib_xmit (unit: `bytes`)
- `ib_total_pkts` = ib_recv_pkts + ib_xmit_pkts (unit: `packets`)

The collector adds a `device` tag to all metrics
