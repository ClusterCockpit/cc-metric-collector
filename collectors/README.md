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

# Installation
Only the `likwidMetric.go` requires preparation steps. For this, the `Makefile` can be used. The LIKWID build needs to be configured:
* Version of LIKWID in `LIKWID_VERSION`
* Target user for LIKWID's accessdaemon in `DAEMON_USER`. The user has to have enough permissions to read the `msr` and `pci` device files
* Target group for LIKWID's accessdaemon in `DAEMON_GROUP`

It performs the following steps:
* Download LIKWID tarball
* Unpacking
* Adjusting configuration for LIKWID build
* Build it
* Copy all required files into `collectors/likwid`
* The accessdaemon is installed with the suid bit set using `sudo` also into `collectors/likwid`
