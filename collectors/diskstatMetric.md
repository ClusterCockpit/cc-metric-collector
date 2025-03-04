## `diskstat` collector

```json
  "diskstat": {
    "exclude_metrics": [
      "part_max_used"
    ],
    "only_metrics": [
      "disk_free",
    ],
    "exclude_mounts": [
      "slurm-tmpfs"
    ]
  }
```

The `diskstat` collector reads data from `/proc/self/mounts` and outputs a handful **node** metrics. 
Any mount point containing one of the strings specified in `exclude_mounts` will be skipped during metric collection.

Both filtering mechanisms are supported:
- `exclude_metrics`: Excludes the specified metrics.
- `only_metrics`: If provided, only the listed metrics are collected. This takes precedence over `exclude_metrics`.

Metrics per device (with `device` tag):
- `disk_total` (unit `GBytes`)
- `disk_free` (unit `GBytes`)

Global metrics:
- `part_max_used` (unit `percent`)
