
## `likwid` collector

The `likwid` collector is probably the most complicated collector. The LIKWID library is included as static library with *direct* access mode. The *direct* access mode is suitable if the daemon is executed by a root user. The static library does not contain the performance groups, so all information needs to be provided in the configuration.

```json
  "likwid": {
    "force_overwrite" : false,
    "invalid_to_zero" : false,
    "eventsets": [
      {
        "events" : {
          "COUNTER0": "EVENT0",
          "COUNTER1": "EVENT1",
        },
        "metrics" : [
          {
            "name": "sum_01",
            "calc": "COUNTER0 + COUNTER1",
            "publish": false,
            "unit": "myunit",
            "type": "cpu"
          }
        ]
      }
    ]
    "globalmetrics" : [
      {
        "name": "global_sum",
        "calc": "sum_01",
        "publish": true,
        "unit": "myunit",
        "type": "cpu"
      }
    ]
  }
```

The `likwid` configuration consists of two parts, the `eventsets` and `globalmetrics`:
- An event set list itself has two parts, the `events` and a set of derivable `metrics`. Each of the `events` is a `counter:event` pair in LIKWID's syntax. The `metrics` are a list of formulas to derive the metric value from the measurements of the `events`' values. Each metric has a name, the formula, a type and a publish flag. There is an optional `unit` field. Counter names can be used like variables in the formulas, so `PMC0+PMC1` sums the measurements for the both events configured in the counters `PMC0` and `PMC1`. You can optionally use `time` for the measurement time and `inverseClock` for `1.0/baseCpuFrequency`. The type tells the LikwidCollector whether it is a metric for each hardware thread (`cpu`) or each CPU socket (`socket`). You may specify a unit for the metric with `unit`. The last one is the publishing flag. It tells the LikwidCollector whether a metric should be sent to the router or is only used internally to compute a global metric.
- The `globalmetrics` are metrics which require data from multiple event set measurements to be derived. The inputs are the metrics in the event sets. Similar to the metrics in the event sets, the global metrics are defined by a name, a formula, a scope and a publish flag. See event set metrics for details. The only difference is that there is no access to the raw event measurements anymore but only to the metrics. Also `time` and `inverseClock` cannot be used anymore. So, the idea is to derive a metric in the `eventsets` section and reuse it in the `globalmetrics` part. If you need a metric only for deriving the global metrics, disable forwarding of the event set metrics (`"publish": false`). **Be aware** that the combination might be misleading because the "behavior" of a metric changes over time and the multiple measurements might count different computing phases. Similar to the metrics in the eventset, you can specify a metric unit with the `unit` field.

Additional options:
- `force_overwrite`: Same as setting `LIKWID_FORCE=1`. In case counters are already in-use, LIKWID overwrites their configuration to do its measurements
- `invalid_to_zero`: In some cases, the calculations result in `NaN` or `Inf`. With this option, all `NaN` and `Inf` values are replaces with `0.0`. See below in [seperate section](./likwidMetric.md#invalid_to_zero-option)
- `access_mode`: Specify LIKWID access mode: `direct` for direct register access as root user or `accessdaemon`. The access mode `perf_event` is current untested.
- `accessdaemon_path`: Folder of the accessDaemon `likwid-accessD` (like `/usr/local/sbin`)
- `liblikwid_path`: Location of `liblikwid.so` including file name like `/usr/local/lib/liblikwid.so`

### Available metric scopes

Hardware performance counters are scattered all over the system nowadays. A counter coveres a specific part of the system. While there are hardware thread specific counter for CPU cycles, instructions and so on, some others are specific for a whole CPU socket/package. To address that, the LikwidCollector provides the specification of a `type` for each metric.

- `cpu` : One metric per CPU hardware thread with the tags `"type" : "cpu"` and `"type-id" : "$cpu_id"`
- `socket` : One metric per CPU socket/package with the tags `"type" : "socket"` and `"type-id" : "$socket_id"`

**Note:** You should not specify the `socket` type for a metric that is measured at `cpu` scope and vice versa, so some kind of expert knowledge or lookup work in the [Likwid Wiki](https://github.com/RRZE-HPC/likwid/wiki) is required. Get the scope of each counter from the *Architecture* pages and as soon as one counter in a metric is socket-specific, the whole metric is socket-specific.

As a guideline:
- All counters `FIXCx`, `PMCy` and `TMAz` have the scope `cpu`
- All counters names containing `BOX` have the scope `socket`
- All `PWRx` counters have scope `socket`, except `"PWR1" : "RAPL_CORE_ENERGY"` has `cpu` scope (AMD Zen)
- All `DFCx` counters have scope `socket`

### Help with the configuration

The configuration for the `likwid` collector is quite complicated. Most users don't use LIKWID with the event:counter notation but rely on the performance groups defined by the LIKWID team for each architecture. In order to help with the `likwid` collector configuration, we included a script `scripts/likwid_perfgroup_to_cc_config.py` that creates the configuration of an `eventset` from a performance group (using a LIKWID installation in `$PATH`):
```
$ likwid-perfctr -i
[...]
short name: ICX
[...]
$ likwid-perfctr -a
[...]
MEM_DP
MEM
FLOPS_SP
CLOCK
[...]
$ scripts/likwid_perfgroup_to_cc_config.py ICX MEM_DP
{
  "events": {
    "FIXC0": "INSTR_RETIRED_ANY",
    "FIXC1": "CPU_CLK_UNHALTED_CORE",
    "..." : "..."
  },
  "metrics" : [
    {
      "calc": "time",
      "name": "Runtime (RDTSC) [s]",
      "publish": true,
      "unit": "seconds"
      "scope": "cpu"
    },
    {
      "..." : "..."
    }
  ]
}
```

You can copy this JSON and add it to the `eventsets` list. If you specify multiple event sets, you can add globally derived metrics in the extra `global_metrics` section with the metric names as variables.

### Mixed usage between daemon and users

LIKWID checks the file `/var/run/likwid.lock` before performing any interfering operations. Who is allowed to access the counters is determined by the owner of the file. If it does not exist, it is created for the current user. So, if you want to temporarly allow counter access to a user (e.g. in a job):

Before (SLURM prolog, ...)
```
$ chown $JOBUSER /var/run/likwid.lock
```

After (SLURM epilog, ...)
```
$ chown $CCUSER /var/run/likwid.lock
```

### `invalid_to_zero` option
In some cases LIKWID returns `0.0` for some events that are further used in processing and maybe used as divisor in a calculation. After evaluation of a metric, the result might be `NaN` or `+-Inf`. These resulting metrics are commonly not created and forwarded to the router because the [InfluxDB line protocol](https://docs.influxdata.com/influxdb/cloud/reference/syntax/line-protocol/#float) does not support these special floating-point values. If you want to have them sent, this option forces these metric values to be `0.0` instead.

One might think this does not happen often but often used metrics in the world of performance engineering like Instructions-per-Cycle (IPC) or more frequently the actual CPU clock are derived with events like `CPU_CLK_UNHALTED_CORE` (Intel) which do not increment in halted state (as the name implies). In there are different power management systems in a chip which can cause a hardware thread to go in such a state. Moreover, if no cycles are executed by the core, also many other events are not incremented as well (like `INSTR_RETIRED_ANY` for retired instructions and part of IPC).


### Example configuration

#### AMD Zen3

```json
  "likwid": {
    "force_overwrite" : false,
    "invalid_to_zero" : false,
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
            "type": "cpu",
            "publish": true
          },
          {
            "name": "flops_any",
            "calc": "0.000001*PMC2/time",
            "unit": "MFlops/s",
            "type": "cpu",
            "publish": true
          },
          {
            "name": "clock",
            "calc": "0.000001*(FIXC1/FIXC2)/inverseClock",
            "type": "cpu",
            "unit": "MHz",
            "publish": true
          },
          {
            "name": "mem1",
            "calc": "0.000001*(DFC0+DFC1+DFC2+DFC3)*64.0/time",
            "unit": "Mbyte/s",
            "type": "socket",
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
            "unit": "Watt"
            "type": "socket",
            "publish": true
          },
          {
            "name": "pwr_pkg",
            "calc": "PWR1/time",
            "type": "socket",
            "unit": "Watt"
            "publish": true
          },
          {
            "name": "mem2",
            "calc": "0.000001*(DFC0+DFC1+DFC2+DFC3)*64.0/time",
            "unit": "Mbyte/s",
            "type": "socket",
            "publish": false
          }
        ]
      }
    ],
    "globalmetrics": [
      {
        "name": "mem_bw",
        "calc": "mem1+mem2",
        "type": "socket",
        "unit": "Mbyte/s",
        "publish": true
      }
    ]
  }
```

