
## `likwid` collector

The `likwid` collector is probably the most complicated collector. The LIKWID library is included as static library with *direct* access mode. The *direct* access mode is suitable if the daemon is executed by a root user. The static library does not contain the performance groups, so all information needs to be provided in the configuration.

The `likwid` configuration consists of two parts, the "eventsets" and "globalmetrics":
- An event set list itself has two parts, the "events" and a set of derivable "metrics". Each of the "events" is a counter:event pair in LIKWID's syntax. The "metrics" are a list of formulas to derive the metric value from the measurements of the "events". Each metric has a name, the formula, a scope and a publish flag. A counter names can be used like variables in the formulas, so `PMC0+PMC1` sums the measurements for the both events configured in the counters `PMC0` and `PMC1`. The scope tells the Collector whether it is a metric for each hardware thread (`cpu`) or each CPU socket (`socket`). The last one is the publishing flag. It tells the collector whether a metric should be sent to the router.
- The global metrics are metrics which require data from all event set measurements to be derived. The inputs are the metrics in the event sets. Similar to the metrics in the event sets, the global metrics are defined by a name, a formula, a scope and a publish flag. See event set metrics for details. The only difference is that there is no access to the raw event measurements anymore but only to the metrics. So, the idea is to derive a metric in the "eventsets" section and reuse it in the "globalmetrics" part. If you need a metric only for deriving the global metrics, disable forwarding of the event set metrics. **Be aware** that the combination might be misleading because the "behavior" of a metric changes over time and the multiple measurements might count different computing phases.

Additional options:
- `force_overwrite`: Same as setting `LIKWID_FORCE=1`. In case counters are already in-use, LIKWID overwrites their configuration to do its measurements
- `invalid_to_zero`: In some cases, the calculations result in `NaN` or `Inf`. With this option, all `NaN` and `Inf` values are replaces with `0.0`.

### Available metric scopes

Hardware performance counters are scattered all over the system nowadays. A counter coveres a specific part of the system. While there are hardware thread specific counter for CPU cycles, instructions and so on, some others are specific for a whole CPU socket/package. To address that, the collector provides the specification of a 'scope' for each metric.

- `cpu` : One metric per CPU hardware thread with the tags `"type" : "cpu"` and `"type-id" : "$cpu_id"`
- `socket` : One metric per CPU socket/package with the tags `"type" : "socket"` and `"type-id" : "$socket_id"`

**Note:** You cannot specify `socket` scope for a metric that is measured at `cpu` scope, so some kind of expert knowledge or lookup work in the [Likwid Wiki](https://github.com/RRZE-HPC/likwid/wiki) is required. Get the scope of each counter from the *Architecture* pages and as soon as one counter in a metric is socket-specific, the whole metric is socket-specific.

As a guideline:
- All counters `FIXCx`, `PMCy` and `TMAz` have the scope `cpu`
- All counters names containing `BOX` have the scope `socket`
- All `PWRx` counters have scope `socket`, except `"PWR1" : "RAPL_CORE_ENERGY"` has `cpu` scope
- All `DFCx` counters have scope `socket`


### Example configuration


```json
  "likwid": {
    "force_overwrite" : false,
    "nan_to_zero" : false,
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
            "scope": "cpu",
            "publish": true
          },
          {
            "name": "flops_any",
            "calc": "0.000001*PMC2/time",
            "scope": "cpu",
            "publish": true
          },
          {
            "name": "clock_mhz",
            "calc": "0.000001*(FIXC1/FIXC2)/inverseClock",
            "scope": "cpu",
            "publish": true
          },
          {
            "name": "mem1",
            "calc": "0.000001*(DFC0+DFC1+DFC2+DFC3)*64.0/time",
            "scope": "socket",
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
            "scope": "socket",
            "publish": true
          },
          {
            "name": "pwr_pkg",
            "calc": "PWR1/time",
            "scope": "socket",
            "publish": true
          },
          {
            "name": "mem2",
            "calc": "0.000001*(DFC0+DFC1+DFC2+DFC3)*64.0/time",
            "scope": "socket",
            "publish": false
          }
        ]
      }
    ],
    "globalmetrics": [
      {
        "name": "mem_bw",
        "calc": "mem1+mem2",
        "scope": "socket",
        "publish": true
      }
    ]
  }
```

### How to get the eventsets and metrics from LIKWID

The `likwid` collector reads hardware performance counters at a **cpu** and **socket** level. The configuration looks quite complicated but it is basically copy&paste from [LIKWID's performance groups](https://github.com/RRZE-HPC/likwid/tree/master/groups). The collector made multiple iterations and tried to use the performance groups but it lacked flexibility. The current way of configuration provides most flexibility.

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
                                 ->       "scope": "cpu",
                                 ->       "publish": true
                                 ->     }
                                 ->   ]
```

