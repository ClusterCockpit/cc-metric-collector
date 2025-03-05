## `numastat` collector

```json
  "numastats": {
    "send_abs_values": true,
    "send_diff_values": true,
    "send_derived_values": true,
    "exclude_metrics": [],
    "only_metrics": []
  }
```

The `numastat` collector reads data from `/sys/devices/system/node/node*/numastat` and outputs a handful **memoryDomain** metrics. See: <https://www.kernel.org/doc/html/latest/admin-guide/numastat.html>

Both filtering mechanisms are supported:
- `exclude_metrics`: Excludes the specified metrics.
- `only_metrics`: If provided, only the listed metrics are collected. This takes precedence over `exclude_metrics`.

Metrics are categorized as follows:

**Absolute Metrics:** (unit: `count`)
- `numastats_numa_hit`: A process wanted to allocate memory from this node, and succeeded.
- `numastats_numa_miss`: A process wanted to allocate memory from another node, but ended up with memory from this node.
- `numastats_numa_foreign`: A process wanted to allocate on this node, but ended up with memory from another node.
- `numastats_local_node`: A process ran on this node's CPU, and got memory from this node.
- `numastats_other_node`: A process ran on a different node's CPU, and got memory from this node.
- `numastats_interleave_hit`: Interleaving wanted to allocate from this node and succeeded.

**Diff Metrics:** (unit: `count`)
- `numastats_numa_hit_diff`
- `numastats_numa_miss_diff`
- `numastats_numa_foreign_diff`
- `numastats_local_node_diff`
- `numastats_other_node_diff`
- `numastats_interleave_hit_diff`

**Derived Metrics:** (unit: `counts/s`)
- `numastats_numa_hit_rate`
- `numastats_numa_miss_rate`
- `numastats_numa_foreign_rate`
- `numastats_local_node_rate`
- `numastats_other_node_rate`
- `numastats_interleave_hit_rate`
