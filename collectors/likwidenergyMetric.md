## `likwidenergy` collector

In contrast to the more general [`likwid` collector](./likwidMetric.md), this collector just reads the RAPL counters to provide energy metrics. In contrast to the `likwid` collector, this collector keeps the energy counters running the whole time and not just of a measurement interval. It covers all RAPL domains (`PKG`, `DRAM`, `PP0`, `PP1`, ...). Depending whether the domain is per socket, per L3 segment or per core, metrics are read and send.

```json
{
    "likwidenergy" : {
        "liblikwid_path" : "/path/to/liblikwid.so",
        "accessdaemon_path" : "/folder/that/contains/likwid-accessD",
        "access_mode" : "direct or accessdaemon",
        "send_difference": true,
        "send_absolute": true
    }
}
```

The first three entries (`liblikwid_path`, `accessdaemon_path` and `access_mode`) are required to set up the access to the RAPL counters. The `access_mode` = `perf_event` is not supported at the moment.

With `send_differences` the difference to the last measurement is provided to the system. With `send_absolute`, the absolute value since start of the system is submitted as metric. It reads the counter at initialization and then updates the value after each measurement.

