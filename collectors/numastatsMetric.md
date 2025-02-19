
## `numastat` collector

```json
  "numastats": {
    "send_abs_values" : true,
    "send_derived_values" : true
}
```

The `numastat` collector reads data from `/sys/devices/system/node/node*/numastat` and outputs a handful **memoryDomain** metrics. See: <https://www.kernel.org/doc/html/latest/admin-guide/numastat.html>

Metrics:

* `numastats_numa_hit`: A process wanted to allocate memory from this node, and succeeded.
* `numastats_numa_miss`: A process wanted to allocate memory from another node, but ended up with memory from this node.
* `numastats_numa_foreign`: A process wanted to allocate on this node, but ended up with memory from another node.
* `numastats_local_node`: A process ran on this node's CPU, and got memory from this node.
* `numastats_other_node`: A process ran on a different node's CPU, and got memory from this node.
* `numastats_interleave_hit`: Interleaving wanted to allocate from this node and succeeded.
* `numastats_numa_hit_rate` (if `send_derived_values == true`): Derived rate value per second.
* `numastats_numa_miss_rate` (if `send_derived_values == true`): Derived rate value per second.
* `numastats_numa_foreign_rate` (if `send_derived_values == true`): Derived rate value per second.
* `numastats_local_node_rate` (if `send_derived_values == true`): Derived rate value per second.
* `numastats_other_node_rate` (if `send_derived_values == true`): Derived rate value per second.
* `numastats_interleave_hit_rate` (if `send_derived_values == true`): Derived rate value per second.
