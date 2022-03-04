## `BeeGFS on Demand` collector
This Collector is to collect BeeGFS on Demand (BeeOND) storage stats.

```json
  "beegfs_storage": {
	"beegfs_path": "/usr/bin/beegfs-ctl",
    "exclude_filesystem": [
      "/mnt/ignore_me"
    ],
    "exclude_metrics": [     
          "ack",
		  "storInf",
		  "unlnk"
  }
```

The `BeeGFS On Demand (BeeOND)` collector uses the `beegfs-ctl` command to read performance metrics for BeeGFS filesystems.

The reported filesystems can be filtered with the `exclude_filesystem` option
in the configuration.

The path to the `beegfs-ctl` command can be configured with the `beegfs_path` option
in the configuration.

When using the `exclude_metrics` option, the excluded metrics are summed as `other`.

Important: The metrics listed below, are similar to the naming of BeeGFS. The Collector prefixes these with `beegfs_cstorage_`(beegfs client meta).
For example beegfs metric `open`-> `beegfs_cstorage_`

Note: BeeGFS FS offers many Metadata Information. Probably it makes sense to exlcude most of them. Nevertheless, these excluded metrics will be summed as `beegfs_cstorage_other`. 

Available Metrics:

* "sum"
* "ack"
* "sChDrct" 
* "getFSize"
* "sAttr"
* "statfs"
* "trunc"
* "close"
* "fsync"
* "ops-rd"
* "MiB-rd/s" 
* "ops-wr"
* "MiB-wr/s" 
* "endbg" 
* "hrtbeat"
* "remNode"
* "storInf"
* "unlnk"


The collector adds a `filesystem` tag to all metrics