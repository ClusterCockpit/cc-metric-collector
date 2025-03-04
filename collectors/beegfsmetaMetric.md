## `BeeGFS on Demand` collector
This Collector is to collect BeeGFS on Demand (BeeOND) metadata clientstats.

```json
  "beegfs_meta": {
	"beegfs_path": "/usr/bin/beegfs-ctl",
    "exclude_filesystem": [
      "/mnt/ignore_me"
    ],
    "exclude_metrics": [     
          "ack",
          "entInf",
          "fndOwn"
    ]
  }
```

The `BeeGFS On Demand (BeeOND)` collector uses the `beegfs-ctl` command to read performance metrics for
BeeGFS filesystems.

The reported filesystems can be filtered with the `exclude_filesystem` option
in the configuration.

The path to the `beegfs-ctl` command can be configured with the `beegfs_path` option
in the configuration.

When using the `exclude_metrics` option, the excluded metrics are summed as `other`.

Important: The metrics listed below are similar to the naming of BeeGFS. The Collector prefixes these with `beegfs_cstorage`(beegfs client storage).

For example `beegfs` metric `open`-> `beegfs_cstorage_open`

Available Metrics:

- sum
- ack
- close
- entInf
- fndOwn
- mkdir
- create
- rddir
- refrEnt
- mdsInf
- rmdir
- rmLnk
- mvDirIns
- mvFiIns
- open
- ren
- sChDrct
- sAttr
- sDirPat
- stat
- statfs
- trunc
- symlnk
- unlnk
- lookLI
- statLI
- revalLI
- openLI
- createLI
- hardlnk
- flckAp
- flckEn
- flckRg
- dirparent
- listXA
- getXA
- rmXA
- setXA
- mirror

The collector adds a `filesystem` tag to all metrics
