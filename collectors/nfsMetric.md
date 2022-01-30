## `nfs3stat` collector

```json
  "nfs3stat": {
    "nfsutils" : "/usr/bin/nfsstat",
    "exclude_metrics": [
      "nfs3_total"
    ]
  }
```

The `nfs3stat` collector reads data from `nfsstat` and outputs a handful **node** metrics. If a metric is not required, it can be excluded from forwarding it to the sink. It is not possible to get data per NFSv3 mount point at the moment.

Metrics:
-`nfs3_total`
-`nfs3_null`
-`nfs3_getattr`
-`nfs3_setattr`
-`nfs3_lookup`
-`nfs3_access`
-`nfs3_readlink`
-`nfs3_read`
-`nfs3_write`
-`nfs3_create`
-`nfs3_mkdir`
-`nfs3_symlink`
-`nfs3_remove`
-`nfs3_rmdir`
-`nfs3_rename`
-`nfs3_link`
-`nfs3_readdir`
-`nfs3_readdirplus`
-`nfs3_fsstat`
-`nfs3_fsinfo`
-`nfs3_pathconf`
-`nfs3_commit`


## `nfs4stat` collector

```json
  "nfs4stat": {
    "nfsutils" : "/usr/bin/nfsstat",
    "exclude_metrics": [
      "nfs4_total"
    ]
  }
```

The `nfs4stat` collector reads data from `nfsstat` and outputs a handful **node** metrics. If a metric is not required, it can be excluded from forwarding it to the sink. It is not possible to get data per NFSv4 mount point at the moment.

Metrics:
-`nfs4_total` 
-`nfs4_null` 
-`nfs4_read` 
-`nfs4_write` 
-`nfs4_commit` 
-`nfs4_open` 
-`nfs4_open_conf` 
-`nfs4_open_noat` 
-`nfs4_open_dgrd` 
-`nfs4_close` 
-`nfs4_setattr` 
-`nfs4_fsinfo` 
-`nfs4_renew` 
-`nfs4_setclntid` 
-`nfs4_confirm` 
-`nfs4_lock` 
-`nfs4_lockt` 
-`nfs4_locku` 
-`nfs4_access` 
-`nfs4_getattr` 
-`nfs4_lookup` 
-`nfs4_lookup_root` 
-`nfs4_remove` 
-`nfs4_rename` 
-`nfs4_link` 
-`nfs4_symlink` 
-`nfs4_create` 
-`nfs4_pathconf` 
-`nfs4_statfs` 
-`nfs4_readlink` 
-`nfs4_readdir` 
-`nfs4_server_caps` 
-`nfs4_delegreturn` 
-`nfs4_getacl` 
-`nfs4_setacl` 
-`nfs4_rel_lkowner` 
-`nfs4_exchange_id` 
-`nfs4_create_session` 
-`nfs4_destroy_session` 
-`nfs4_sequence` 
-`nfs4_get_lease_time` 
-`nfs4_reclaim_comp` 
-`nfs4_secinfo_no` 
-`nfs4_bind_conn_to_ses` 
