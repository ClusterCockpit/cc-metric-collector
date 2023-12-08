
## `ibstat` collector

```json
  "ibstat": {
    "exclude_devices": [
      "mlx4"
    ],
    "send_abs_values": true,
    "send_derived_values": true
  }
```

The `ibstat` collector includes all Infiniband devices that can be
found below `/sys/class/infiniband/` and where any of the ports provides a
LID file (`/sys/class/infiniband/<dev>/ports/<port>/lid`)

The devices can be filtered with the `exclude_devices` option in the configuration.

For each found LID the collector reads data through the sysfs files below `/sys/class/infiniband/<device>`. (See: <https://www.kernel.org/doc/Documentation/ABI/stable/sysfs-class-infiniband>)

Metrics:

* `ib_recv`
* `ib_xmit`
* `ib_recv_pkts`
* `ib_xmit_pkts`
* `ib_total = ib_recv + ib_xmit` (if `send_total_values == true`)
* `ib_total_pkts = ib_recv_pkts + ib_xmit_pkts` (if `send_total_values == true`)
* `ib_recv_bw` (if `send_derived_values == true`)
* `ib_xmit_bw` (if `send_derived_values == true`)
* `ib_recv_pkts_bw` (if `send_derived_values == true`)
* `ib_xmit_pkts_bw` (if `send_derived_values == true`)

The collector adds a `device` tag to all metrics
