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

# Available collectors

* [`cpustat`](./cpustatMetric.md)
* [`memstat`](./memstatMetric.md)
* [`diskstat`](./diskstatMetric.md)
* [`loadavg`](./loadavgMetric.md)
* [`netstat`](./netstatMetric.md)
* [`ibstat`](./infinibandMetric.md)
* [`tempstat`](./tempMetric.md)
* [`lustre`](./lustreMetric.md)
* [`likwid`](./likwidMetric.md)
* [`nvidia`](./nvidiaMetric.md)
* [`customcmd`](./customCmdMetric.md)
* [`ipmistat`](./ipmiMetric.md)
* [`topprocs`](./topprocsMetric.md)

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


