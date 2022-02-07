
## `nfs3stat` collector

```json
  "nfs3stat": {
    "nfsstat" : "/path/to/nfsstat",
    "exclude_metrics": [
      "nfs3_total"
    ]
  }
```

The `nfs3stat` collector reads data from `nfsstat` command and outputs a handful **node** metrics. If a metric is not required, it can be excluded from forwarding it to the sink. There is currently no possibility to get the metrics per mount point.


Metrics:
* `nfs3_total` 
* `nfs3_null` 
* `nfs3_getattr` 
* `nfs3_setattr` 
* `nfs3_lookup` 
* `nfs3_access` 
* `nfs3_readlink` 
* `nfs3_read` 
* `nfs3_write` 
* `nfs3_create` 
* `nfs3_mkdir` 
* `nfs3_symlink` 
* `nfs3_remove` 
* `nfs3_rmdir` 
* `nfs3_rename` 
* `nfs3_link` 
* `nfs3_readdir` 
* `nfs3_readdirplus` 
* `nfs3_fsstat` 
* `nfs3_fsinfo` 
* `nfs3_pathconf` 
* `nfs3_commit` 

