This folder contains the collectors for the cc-metric-collector.

# `metricCollector.go`
The base class/configuration is located in `metricCollector.go`.

# Collectors

* `memstatMetric.go`: Reads `/proc/meminfo` to calculate the **node** metric `mem_used`
* `loadavgMetric.go`: Reads `/proc/loadavg` and submits the **node** metrics:
   * `load_one`
   * `load_five`
   * `load_fifteen`
* `netstatMetric.go`: Reads `/proc/net/dev` and submits for all network devices (except loopback `lo`) the **node** metrics:
   * `<dev>_bytes_in`
   * `<dev>_bytes_out`
   * `<dev>_pkts_in`
   * `<dev>_pkts_out`
* `lustreMetric.go`: Reads Lustre's stats file `/proc/fs/lustre/llite/lnec-XXXXXX/stats` and submits the **node** metrics:
   * `read_bytes`
   * `read_requests`
   * `write_bytes`
   * `write_bytes`
   * `open`
   * `close`
   * `getattr`
   * `setattr`
   * `statfs`
   * `inode_permission`
* `infinibandMetric.go`: Reads InfiniBand LID from `/sys/class/infiniband/mlx4_0/ports/1/lid` and uses the `perfquery` command to read the **node** metrics:
   * `ib_recv`
   * `ib_xmit`
* `likwidMetric.go`: Reads hardware performance events using LIKWID. It submits **socket** and **cpu** metrics:
   * `mem_bw` (socket)
   * `power` (socket, Sum of RAPL domains PKG and DRAM)
   * `flops_dp` (cpu)
   * `flops_sp` (cpu)
   * `flops_any` (cpu, `2*flops_dp + flops_sp`)
   * `cpi` (cpu)
   * `clock` (cpu)
* `cpustatMetric.go`: Read CPU specific values from `/proc/stat`
* `topprocsMetric.go`: Reads the Top5 processes by their CPU usage
* `nvidiaMetric.go`: Read data about Nvidia GPUs using the NVML library

If any of the collectors cannot be initialized, it is excluded from all further reads. Like if the Lustre stat file is not a valid path, no Lustre specific metrics will be recorded.

# InfiniBand collector
The InfiniBand collector requires the LID file to read the data. It has to be configured in the collector itself (`LIDFILE` in `infinibandMetric.go`)

# Lustre collector
The Lustre collector requires the path to the Lustre stats file. It has to be configured in the collector itself (`LUSTREFILE` in `lustreMetric.go`)

# LIKWID collector
The `likwidMetric.go` requires preparation steps. For this, the `Makefile` can be used.

There two ways to configure the LIKWID build: use a central installation of LIKWID or build a fresh copy. This can be controlled with `CENTRAL_INSTALL = <true|false>`.

If `CENTRAL_INSTALL = true`:
* Set the `LIKWID_BASE` to the base folder of LIKWID (try `echo $(realpath $(dirname $(which likwid-topology))/..)`)
* Set the `LIKWID_VERSION` to a related LIKWID version. At least similar minor release 5.0.x or 5.1.x.

If `CENTRAL_INSTALL = false`:
* Version of LIKWID in `LIKWID_VERSION` to download from official FTP server
* Target user for LIKWID's accessdaemon in `DAEMON_USER`. The user has to have enough permissions to read the `msr` and `pci` device files
* Target group for LIKWID's accessdaemon in `DAEMON_GROUP`
* **No** need to change `LIKWID_BASE`!

Calling `make` performs the following steps:
* Download LIKWID tarball
* Unpacking
* Adjusting configuration to build LIKWID as static library
* Build it
* Copy all required files into `collectors/likwid`
* If `CENTRAL_INSTALL = false`, the accessdaemon is installed with the suid bit set using `sudo` with the configured `DAEMON_USER` and `DAEMON_GROUP`.
* Adjust group path in LIKWID collector

## Custom metrics for LIKWID
The `likwidMetric.go` collector uses it's own performance group tree by copying it from the LIKWID sources. By adding groups to this directory tree, you can use them in the collector. Additionally, you have to tell the collector which group to measure and which event count or derived metric should be used.

The collector contains a hash map with the groups and metrics (reduced set of metrics):
```
var likwid_metrics = map[string][]LikwidMetric{
	"MEM_DP": {LikwidMetric{name: "mem_bw", search: "Memory bandwidth [MBytes/s]", socket_scope: true}},
	"FLOPS_SP": {LikwidMetric{name: "clock", search: "Clock [MHz]", socket_scope: false}},
}
```

The collector will measure both groups `MEM_DP` and `FLOPS_SP` for `duration` seconds (global `config.json`). It matches the LIKWID name by using the `search` string and submits the value with the given `name` as field name in either the `socket` or the `cpu` metric depending on the `socket_scope` flag.

## Todos
* Aggregate a per-hwthread metric to a socket metric if `socket_scope=true`
* Add a JSON configuration file `likwid.json` and suitable reader for the metrics and group tree path.
  * Do we need separate sections for CPU architectures? (one config file for all architectures?)
  * How to encode postprocessing steps. There are Go packages like [eval](https://github.com/apaxa-go/eval) or [govaluate](https://github.com/Knetic/govaluate) but they seem to be not maintained anymore.

# Contributing own collectors
A collector reads data from any source, parses it to metrics and submits these metrics to the `metric-collector`. A collector provides three function:

* `Init() error`: Initializes the collector and its data structures.
* `Read(duration time.Duration) error`: Read, parse and submit data. If the collector has to measure anything for some duration, use the provided function argument `duration`
* `Close()`: Closes down the collector.

It is recommanded to call `setup()` in the `Init()` function as it creates the required data structures.

Each collector contains data structures for the submission of metrics after calling `setup()` in `Init()`:

* `node` (`map[string]string`): Just key-value store for all metrics concerning the whole system
* `sockets` (`map[int]map[string]string`): One key-value store per CPU socket like `sockets[1]["testmetric] = 1.0` for the second socket. You can either use `len(sockets)` to get the amount of sockets or you use `SocketList()`.
* `cpus` (`map[int]map[string]string`): One key-value store per hardware thread like `cpus[12]["testmetric] = 1.0`. You can either use `len(cpus)` to get the amount of hardware threads or you use `CpuList()`.

Finally, the collector needs to be registered in the `metric-collector.go`. There is a list of collectors called `Collectors` which is a map (string -> pointer to collector). Add a new entry with a descriptive name and the new collector.
