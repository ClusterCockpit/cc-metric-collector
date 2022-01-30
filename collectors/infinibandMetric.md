
## `ibstat` collector

```json
  "ibstat": {
    "perfquery_path" : "<path to perfquery command>",
    "exclude_devices": [
      "mlx4"
    ]
  }
```

The `ibstat` collector reads either data through the `perfquery` command or the sysfs files below `/sys/class/infiniband/<device>`.

Metrics:
* `ib_recv`
* `ib_xmit`

The collector adds a `device` tag to all metrics
