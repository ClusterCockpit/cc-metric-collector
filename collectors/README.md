# CCMetric collectors

This folder contains the collectors for the cc-metric-collector.

# Configuration

```json
{
    "collector_type" : {
        <collector specific configuration>
    }
}
```

In contrast to the configuration files for sinks and receivers, the collectors configuration is not a list but a set of dicts. This is required because we didn't manage to partially read the type before loading the remaining configuration. We are eager to change this to the same format.


## `memstat` collector

```json
  "memstat": {
    "exclude_metrics": [
      "mem_used"
    ]
  }
```

The `memstat` collector reads data from `/proc/meminfo` and outputs a handful **node** metrics. If a metric is not required, it can be excluded from forwarding it to the sink.


Metrics:
* `mem_total`
* `mem_sreclaimable`
* `mem_slab`
* `mem_free`
* `mem_buffers`
* `mem_cached`
* `mem_available`
* `mem_shared`
* `swap_total`
* `swap_free`
* `mem_used` = `mem_total` - (`mem_free` + `mem_buffers` + `mem_cached`)

## `loadavg` collector
```json
  "loadavg": {
    "exclude_metrics": [
      "proc_run"
    ]
  }
```

The `loadavg` collector reads data from `/proc/loadavg` and outputs a handful **node** metrics. If a metric is not required, it can be excluded from forwarding it to the sink.

Metrics:
* `load_one`
* `load_five`
* `load_fifteen`
* `proc_run`
* `proc_total`

## `netstat` collector
```json
  "netstat": {
    "exclude_devices": [
      "lo"
    ]
  }
```

The `netstat` collector reads data from `/proc/net/dev` and outputs a handful **node** metrics. If a device is not required, it can be excluded from forwarding it to the sink. Commonly the `lo` device should be excluded.

Metrics:
* `bytes_in`
* `bytes_out`
* `pkts_in`
* `pkts_out`

The device name is added as tag `device`.


## `diskstat` collector

```json
  "diskstat": {
    "exclude_metrics": [
      "read_ms"
    ],
  }
```

The `netstat` collector reads data from `/proc/net/dev` and outputs a handful **node** metrics. If a metric is not required, it can be excluded from forwarding it to the sink.

Metrics:
* `reads`
* `reads_merged`
* `read_sectors`
* `read_ms`
* `writes`
* `writes_merged`
* `writes_sectors`
* `writes_ms`
* `ioops`
* `ioops_ms`
* `ioops_weighted_ms`
* `discards`
* `discards_merged`
* `discards_sectors`
* `discards_ms`
* `flushes`
* `flushes_ms`


The device name is added as tag `device`.

## `cpustat` collector
```json
  "netstat": {
    "exclude_metrics": [
      "cpu_idle"
    ]
  }
```

The `cpustat` collector reads data from `/proc/stats` and outputs a handful **node** and **hwthread** metrics. If a metric is not required, it can be excluded from forwarding it to the sink.

Metrics:
* `cpu_user`
* `cpu_nice`
* `cpu_system`
* `cpu_idle`
* `cpu_iowait`
* `cpu_irq`
* `cpu_softirq`
* `cpu_steal`
* `cpu_guest`
* `cpu_guest_nice`

## `ibstat` collector

```json
  "ibstat": {
    "perfquery_path" : "<path to perfquery command>",
    "exclude_devices": [
      "mlx4"
    ]
  }
```

The `ibstat` collector reads either data through the `perfquery` command or the sysfs files below `/sys/class/infiniband/<device>`.

Metrics:
* `ib_recv`
* `ib_xmit`


## `lustrestat` collector

```json
  "lustrestat": {
    "procfiles" : [
      "/proc/fs/lustre/llite/lnec-XXXXXX/stats"
    ],
    "exclude_metrics": [
      "setattr",
      "getattr"
    ]
  }
```

The `lustrestat` collector reads from the procfs stat files for Lustre like `/proc/fs/lustre/llite/lnec-XXXXXX/stats`.

Metrics:
* `read_bytes`
* `read_requests`
* `write_bytes`
* `write_requests`
* `open`
* `close`
* `getattr`
* `setattr`
* `statfs`
* `inode_permission`

## `nvidia` collector

```json
  "lustrestat": {
    "exclude_devices" : [
      "0","1"
    ],
    "exclude_metrics": [
      "fb_memory",
      "fan"
    ]
  }
```

Metrics:
* `util`
* `mem_util`
* `mem_total`
* `fb_memory`
* `temp`
* `fan`
* `ecc_mode`
* `perf_state`
* `power_usage_report`
* `graphics_clock_report`
* `sm_clock_report`
* `mem_clock_report`
* `max_graphics_clock`
* `max_sm_clock`
* `max_mem_clock`
* `ecc_db_error`
* `ecc_sb_error`
* `power_man_limit`
* `encoder_util`
* `decoder_util`

It uses a separate `type` in the metrics. The output metric looks like this:
`<name>,type=accelerator,type-id=<nvidia-gpu-id> value=<metric value> <timestamp>`

## `tempstat` collector

```json
  "lustrestat": {
    "tag_override" : {
        "<device like hwmon1>" : {
            "type" : "socket",
            "type-id" : "0"
        }
    },
    "exclude_metrics": [
      "metric1",
      "metric2"
    ]
  }
```

The `tempstat` collector reads the data from `/sys/class/hwmon/<device>/tempX_{input,label}`

Metrics:
* `temp_*`: The metric name is taken from the `label` files.

## `likwid` collector
```json
  "likwid": {
    "eventsets": [
      {
        "events": {
          "FIXC1": "ACTUAL_CPU_CLOCK",
          "FIXC2": "MAX_CPU_CLOCK",
          "PMC0": "RETIRED_INSTRUCTIONS",
          "PMC1": "CPU_CLOCKS_UNHALTED",
          "PMC2": "RETIRED_SSE_AVX_FLOPS_ALL",
          "PMC3": "MERGE",
          "DFC0": "DRAM_CHANNEL_0",
          "DFC1": "DRAM_CHANNEL_1",
          "DFC2": "DRAM_CHANNEL_2",
          "DFC3": "DRAM_CHANNEL_3"
        },
        "metrics": [
          {
            "name": "ipc",
            "calc": "PMC0/PMC1",
            "socket_scope": false,
            "publish": true
          },
          {
            "name": "flops_any",
            "calc": "0.000001*PMC2/time",
            "socket_scope": false,
            "publish": true
          },
          {
            "name": "clock_mhz",
            "calc": "0.000001*(FIXC1/FIXC2)/inverseClock",
            "socket_scope": false,
            "publish": true
          },
          {
            "name": "mem1",
            "calc": "0.000001*(DFC0+DFC1+DFC2+DFC3)*64.0/time",
            "socket_scope": true,
            "publish": false
          }
        ]
      },
      {
        "events": {
          "DFC0": "DRAM_CHANNEL_4",
          "DFC1": "DRAM_CHANNEL_5",
          "DFC2": "DRAM_CHANNEL_6",
          "DFC3": "DRAM_CHANNEL_7",
          "PWR0": "RAPL_CORE_ENERGY",
          "PWR1": "RAPL_PKG_ENERGY"
        },
        "metrics": [
          {
            "name": "pwr_core",
            "calc": "PWR0/time",
            "socket_scope": false,
            "publish": true
          },
          {
            "name": "pwr_pkg",
            "calc": "PWR1/time",
            "socket_scope": true,
            "publish": true
          },
          {
            "name": "mem2",
            "calc": "0.000001*(DFC0+DFC1+DFC2+DFC3)*64.0/time",
            "socket_scope": true,
            "publish": false
          }
        ]
      }
    ],
    "globalmetrics": [
      {
        "name": "mem_bw",
        "calc": "mem1+mem2",
        "socket_scope": true,
        "publish": true
      }
    ]
  }
```

_Example config suitable for AMD Zen3_

The `likwid` collector reads hardware performance counters at a **hwthread** and **socket** level. The configuration looks quite complicated but it is basically copy&paste from [LIKWID's performance groups](https://github.com/RRZE-HPC/likwid/tree/master/groups). The collector made multiple iterations and tried to use the performance groups but it lacked flexibility. The current way of configuration provides most flexibility.

The logic is as following: There are multiple eventsets, each consisting of a list of counters+events and a list of metrics. If you compare a common performance group with the example setting above, there is not much difference:
```
EVENTSET                         ->   "events": {
FIXC1 ACTUAL_CPU_CLOCK           ->     "FIXC1": "ACTUAL_CPU_CLOCK",
FIXC2 MAX_CPU_CLOCK              ->     "FIXC2": "MAX_CPU_CLOCK",
PMC0  RETIRED_INSTRUCTIONS       ->     "PMC0" : "RETIRED_INSTRUCTIONS",
PMC1  CPU_CLOCKS_UNHALTED        ->     "PMC1" : "CPU_CLOCKS_UNHALTED",
PMC2  RETIRED_SSE_AVX_FLOPS_ALL  ->     "PMC2": "RETIRED_SSE_AVX_FLOPS_ALL",
PMC3  MERGE                      ->     "PMC3": "MERGE",
                                 ->   }
```

The metrics are following the same procedure:

```
METRICS                          ->   "metrics": [
IPC   PMC0/PMC1                  ->     {
                                 ->       "name" : "IPC",
                                 ->       "calc" : "PMC0/PMC1",
                                 ->       "socket_scope": false,
                                 ->       "publish": true
                                 ->     }
                                 ->   ]
```

The `socket_scope` option tells whether it is submitted per socket or per hwthread. If a metric is only used for internal calculations, you can set `publish = false`.

Since some metrics can only be gathered in multiple measurements (like the memory bandwidth on AMD Zen3 chips), configure multiple eventsets like in the example config and use the `globalmetrics` section to combine them. **Be aware** that the combination might be misleading because the "behavior" of a metric changes over time and the multiple measurements might count different computing phases.

## Todos

* [ ] Exclude devices for `diskstat` collector
* [ ] Aggreate metrics to higher topology entity (sum hwthread metrics to socket metric, ...). Needs to be configurable

# Contributing own collectors
A collector reads data from any source, parses it to metrics and submits these metrics to the `metric-collector`. A collector provides three function:

* `Name() string`: Return the name of the collector
* `Init(config json.RawMessage) error`: Initializes the collector using the given collector-specific config in JSON. Check if needed files/commands exists, ...
* `Initialized() bool`: Check if a collector is successfully initialized
* `Read(duration time.Duration, output chan ccMetric.CCMetric)`: Read, parse and submit data to the `output` channel as [`CCMetric`](../internal/ccMetric/README.md). If the collector has to measure anything for some duration, use the provided function argument `duration`. 
* `Close()`: Closes down the collector.

It is recommanded to call `setup()` in the `Init()` function.

Finally, the collector needs to be registered in the `collectorManager.go`. There is a list of collectors called `AvailableCollectors` which is a map (`collector_type_string` -> `pointer to MetricCollector interface`). Add a new entry with a descriptive name and the new collector.


