<!--
---
title: Metric Collectors
description: Metric collectors for cc-metric-collector
categories: [cc-metric-collector]
tags: ['Admin']
weight: 2
hugo_path: docs/reference/cc-metric-collector/collectors/_index.md
---
-->

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
* [`iostat`](./iostatMetric.md)
* [`diskstat`](./diskstatMetric.md)
* [`loadavg`](./loadavgMetric.md)
* [`netstat`](./netstatMetric.md)
* [`ibstat`](./infinibandMetric.md)
* [`tempstat`](./tempMetric.md)
* [`lustrestat`](./lustreMetric.md)
* [`likwid`](./likwidMetric.md)
* [`nvidia`](./nvidiaMetric.md)
* [`customcmd`](./customCmdMetric.md)
* [`ipmistat`](./ipmiMetric.md)
* [`topprocs`](./topprocsMetric.md)
* [`nfs3stat`](./nfs3Metric.md)
* [`nfs4stat`](./nfs4Metric.md)
* [`nfsiostat`](./nfsiostatMetric.md)
* [`cpufreq`](./cpufreqMetric.md)
* [`cpufreq_cpuinfo`](./cpufreqCpuinfoMetric.md)
* [`schedstat`](./schedstatMetric.md)
* [`numastats`](./numastatsMetric.md)
* [`gpfs`](./gpfsMetric.md)
* [`beegfs_meta`](./beegfsmetaMetric.md)
* [`beegfs_storage`](./beegfsstorageMetric.md)
* [`rocm_smi`](./rocmsmiMetric.md)
* [`slurm_cgroup`](./slurmCgroupMetric.md)

## Todos

* [ ] Aggreate metrics to higher topology entity (sum hwthread metrics to socket metric, ...). Needs to be configurable

# Contributing own collectors
A collector reads data from any source, parses it to metrics and submits these metrics to the `metric-collector`. A collector provides three function:

* `Name() string`: Return the name of the collector
* `Init(config json.RawMessage) error`: Initializes the collector using the given collector-specific config in JSON. Check if needed files/commands exists, ...
* `Initialized() bool`: Check if a collector is successfully initialized
* `Read(duration time.Duration, output chan ccMessage.CCMessage)`: Read, parse and submit data to the `output` channel as [`CCMessage`](https://github.com/ClusterCockpit/cc-lib/blob/main/ccMessage/README.md). If the collector has to measure anything for some duration, use the provided function argument `duration`.
* `Close()`: Closes down the collector.

It is recommanded to call `setup()` in the `Init()` function.

Finally, the collector needs to be registered in the `collectorManager.go`. There is a list of collectors called `AvailableCollectors` which is a map (`collector_type_string` -> `pointer to MetricCollector interface`). Add a new entry with a descriptive name and the new collector.

## Sample collector

```go
package collectors

import (
    "encoding/json"
    "time"

    lp "github.com/ClusterCockpit/cc-metric-collector/internal/ccMetric"
)

// Struct for the collector-specific JSON config
type SampleCollectorConfig struct {
    ExcludeMetrics []string `json:"exclude_metrics"`
}

type SampleCollector struct {
    metricCollector
    config SampleCollectorConfig
}

func (m *SampleCollector) Init(config json.RawMessage) error {
    // Check if already initialized
    if m.init {
        return nil
    }

    m.name = "SampleCollector"
    m.setup()
    if len(config) > 0 {
        err := json.Unmarshal(config, &m.config)
        if err != nil {
            return err
        }
    }
    m.meta = map[string]string{"source": m.name, "group": "Sample"}

    m.init = true
    return nil
}

func (m *SampleCollector) Read(interval time.Duration, output chan lp.CCMetric) {
    if !m.init {
        return
    }
    // tags for the metric, if type != node use proper type and type-id
    tags := map[string]string{"type" : "node"}

    x, err := GetMetric()
    if err != nil {
        cclog.ComponentError(m.name, fmt.Sprintf("Read(): %v", err))
    }

    // Each metric has exactly one field: value !
    value := map[string]interface{}{"value": int64(x)}
    if y, err := lp.New("sample_metric", tags, m.meta, value, time.Now()); err == nil {
        output <- y
    }
}

func (m *SampleCollector) Close() {
    m.init = false
    return
}
```
