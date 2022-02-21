
## `ibstat_perfquery` collector

```json
  "ibstat_perfquery": {
    "perfquery_path": "/path/to/perfquery",
    "exclude_devices": [
      "mlx4"
    ]
  }
```

The `ibstat_perfquery` collector includes all Infiniband devices that can be
found below `/sys/class/infiniband/` and where any of the ports provides a
LID file (`/sys/class/infiniband/<dev>/ports/<port>/lid`)

The devices can be filtered with the `exclude_devices` option in the configuration.

For each found LID the collector calls the `perfquery` command. The path to the
`perfquery` command can be configured with the `perfquery_path` option in the configuration

Metrics:
* `ib_recv`
* `ib_xmit`
* `ib_recv_pkts`
* `ib_xmit_pkts`

The collector adds a `device` tag to all metrics
