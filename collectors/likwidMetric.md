
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
