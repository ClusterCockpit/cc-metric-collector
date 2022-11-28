## `nfsiostat` collector

```json
  "nfsiostat": {
    "exclude_metrics": [
      "nfsio_oread"
    ],
    "exclude_filesystems" : [
        "/mnt",
    ],
    "use_server_as_stype": false
  }
```

The `nfsiostat` collector reads data from `/proc/self/mountstats` and outputs a handful **node** metrics for each NFS filesystem. If a metric or filesystem is not required, it can be excluded from forwarding it to the sink.

Metrics:
* `nfsio_nread`: Bytes transferred by normal `read()` calls
* `nfsio_nwrite`: Bytes transferred by normal `write()` calls
* `nfsio_oread`: Bytes transferred by `read()` calls with `O_DIRECT`
* `nfsio_owrite`: Bytes transferred by `write()` calls with `O_DIRECT`
* `nfsio_pageread`: Pages transferred by `read()` calls
* `nfsio_pagewrite`: Pages transferred by `write()` calls
* `nfsio_nfsread`: Bytes transferred for reading from the server
* `nfsio_nfswrite`: Pages transferred by writing to the server

The `nfsiostat` collector adds the mountpoint to the tags as `stype=filesystem,stype-id=<mountpoint>`. If the server address should be used instead of the mountpoint, use the `use_server_as_stype` config setting.