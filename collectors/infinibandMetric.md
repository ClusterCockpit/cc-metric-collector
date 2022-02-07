
## `ibstat` collector

```json
  "ibstat": {
    "exclude_devices": [
      "mlx4"
    ]
  }
```

The `ibstat` includes all devices that can be found below `/sys/class/infiniband/`
and where any of the ports provides a `lid` file (`/sys/class/infiniband/<dev>/ports/<port>/lid`)
The devices can be filtered with the `exclude_devices` option in the configuration.
The collector reads data through the sysfs files below `/sys/class/infiniband/<device>`.

Metrics:
* `ib_recv`
* `ib_xmit`
* `ib_recv_pkts`
* `ib_xmit_pkts`

The collector adds a `device` tag to all metrics
