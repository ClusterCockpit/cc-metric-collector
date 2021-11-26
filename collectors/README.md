This folder contains the collectors for the cc-metric-collector.

# `metricCollector.go`
The base class/configuration is located in `metricCollector.go`.

# Collectors

* `memstatMetric.go`: Reads `/proc/meminfo` to calculate **node** metrics. It also combines values to the metric `mem_used`
* `loadavgMetric.go`: Reads `/proc/loadavg` and submits **node** metrics:
* `netstatMetric.go`: Reads `/proc/net/dev` and submits for all network devices as the **node** metrics.
* `lustreMetric.go`: Reads Lustre's stats files and submits **node** metrics:
* `infinibandMetric.go`: Reads InfiniBand metrics. It uses the `perfquery` command to read the **node** metrics but can fallback to sysfs counters in case `perfquery` does not work.
* `likwidMetric.go`: Reads hardware performance events using LIKWID. It submits **socket** and **cpu** metrics
* `cpustatMetric.go`: Read CPU specific values from `/proc/stat`
* `topprocsMetric.go`: Reads the TopX processes by their CPU usage. X is configurable
* `nvidiaMetric.go`: Read data about Nvidia GPUs using the NVML library
* `tempMetric.go`: Read temperature data from `/sys/class/hwmon/hwmon*`
* `ipmiMetric.go`: Collect data from `ipmitool` or as fallback `ipmi-sensors`
* `customCmdMetric.go`: Run commands or read files and submit the output (output has to be in InfluxDB line protocol!)

If any of the collectors cannot be initialized, it is excluded from all further reads. Like if the Lustre stat file is not a valid path, no Lustre specific metrics will be recorded.

# Collector configuration

```json
  "collectors": [
    "tempstat"
  ],
  "collect_config": {
    "tempstat": {
      "tag_override": {
        "hwmon0" : {
            "type" : "socket",
            "type-id" : "0"
        },
        "hwmon1" : {
            "type" : "socket",
            "type-id" : "1"
        }
      }
    }
  }
```

The configuration of the collectors in the main config files consists of two parts: active collectors (`collectors`) and collector configuration (`collect_config`). At startup, all collectors in the `collectors` list is initialized and, if successfully initialized, added to the active collectors for metric retrieval. At initialization the collector-specific configuration from the `collect_config` section is handed over. Each collector has own configuration options, check at the collector-specific section.

## `memstat`

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

## `loadavg`
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

## `netstat`
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


## `diskstat`

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

## `cpustat`
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

## `likwid`
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

[ ] Exclude devices for `diskstat` collector
[ ] Aggreate metrics to higher topology entity (sum hwthread metrics to socket metric, ...). Needs to be configurable

# Contributing own collectors
A collector reads data from any source, parses it to metrics and submits these metrics to the `metric-collector`. A collector provides three function:

* `Init(config []byte) error`: Initializes the collector using the given collector-specific config in JSON.
* `Read(duration time.Duration, out *[]lp.MutableMetric) error`: Read, parse and submit data to the `out` list. If the collector has to measure anything for some duration, use the provided function argument `duration`. 
* `Close()`: Closes down the collector.

It is recommanded to call `setup()` in the `Init()` function.

Finally, the collector needs to be registered in the `metric-collector.go`. There is a list of collectors called `Collectors` which is a map (string -> pointer to collector). Add a new entry with a descriptive name and the new collector.

## Sample collector

```go
package collectors

import (
    "encoding/json"
    lp "github.com/influxdata/line-protocol"
    "time"
)

// Struct for the collector-specific JSON config
type SampleCollectorConfig struct {
    ExcludeMetrics []string `json:"exclude_metrics"`
}

type SampleCollector struct {
    MetricCollector
    config SampleCollectorConfig
}

func (m *SampleCollector) Init(config []byte) error {
    m.name = "SampleCollector"
    m.setup()
    if len(config) > 0 {
        err := json.Unmarshal(config, &m.config)
        if err != nil {
            return err
        }
    }
    m.init = true
    return nil
}

func (m *SampleCollector) Read(interval time.Duration, out *[]lp.MutableMetric) {
    if !m.init {
        return
    }
    // tags for the metric, if type != node use proper type and type-id
    tags := map[string][string]{"type" : "node"}
    // Each metric has exactly one field: value !
    value := map[string]interface{}{"value": int(x)}
    y, err := lp.New("sample_metric", tags, value, time.Now())
    if err == nil {
        *out = append(*out, y)
    }
}

func (m *SampleCollector) Close() {
    m.init = false
    return
}
```
