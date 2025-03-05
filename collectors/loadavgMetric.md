## `loadavg` collector

```json
  "loadavg": {
    "exclude_metrics": [
      "proc_run"
    ],
    "only_metrics": [
      "load_one",
      "proc_total"
    ]
  }
```

The `loadavg` collector reads data from `/proc/loadavg` and outputs a handful **node** metrics.

Both filtering mechanisms are supported:
- `exclude_metrics`: Excludes the specified metrics.
- `only_metrics`: If provided, only the listed metrics are collected. This takes precedence over `exclude_metrics`.

Metrics:
- `load_one`
- `load_five`
- `load_fifteen`
- `proc_run`
- `proc_total`
