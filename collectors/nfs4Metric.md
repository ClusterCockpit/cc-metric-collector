## `nfs4stat` collector

```json
  "nfs4stat": {
    "nfsstat": "/path/to/nfsstat",
    "exclude_metrics": [
      "total"
    ],
    "only_metrics": [
      "read"
    ],
    "send_abs_values": true,
    "send_diff_values": true,
    "send_derived_values": true
  }
```

The `nfs4stat` collector reads data from `nfsstat` command and outputs a handful **node** metrics. 

Both filtering mechanisms are supported:
- `exclude_metrics`: Excludes the specified metrics. Uses base metric names without the nfs4_ prefix.
- `only_metrics`: If provided, only the listed metrics are collected. This takes precedence over `exclude_metrics`.


**Absolute Metrics:**
- `nfs4_total`
- `nfs4_null`
- `nfs4_read`
- `nfs4_write`
- `nfs4_commit`
- `nfs4_open`
- `nfs4_open_conf`
- `nfs4_open_noat`
- `nfs4_open_dgrd`
- `nfs4_close`
- `nfs4_setattr`
- `nfs4_fsinfo`
- `nfs4_renew`
- `nfs4_setclntid`
- `nfs4_confirm`
- `nfs4_lock`
- `nfs4_lockt`
- `nfs4_locku`
- `nfs4_access`
- `nfs4_getattr`
- `nfs4_lookup`
- `nfs4_lookup_root`
- `nfs4_remove`
- `nfs4_rename`
- `nfs4_link`
- `nfs4_symlink`
- `nfs4_create`
- `nfs4_pathconf`
- `nfs4_statfs`
- `nfs4_readlink`
- `nfs4_readdir`
- `nfs4_server_caps`
- `nfs4_delegreturn`
- `nfs4_getacl`
- `nfs4_setacl`
- `nfs4_rel_lkowner`
- `nfs4_exchange_id`
- `nfs4_create_session`
- `nfs4_destroy_session`
- `nfs4_sequence`
- `nfs4_get_lease_time`
- `nfs4_reclaim_comp`
- `nfs4_secinfo_no`
- `nfs4_bind_conn_to_ses`
- `nfs4_seek`

**Diff Metrics:**
For each metric, if `send_diff_values` is enabled, the collector computes the difference and sends it with the suffix `_diff`.

**Derived Metrics:**
For each metric, if `send_derived_values` is enabled, the collector computes the rate (difference divided by the time interval) and sends it with the suffix `_rate`.
