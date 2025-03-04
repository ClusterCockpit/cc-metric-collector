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

**CPU & Performance**
* [`loadavg`](./loadavgMetric.md) - System load averages
* [`cpustat`](./cpustatMetric.md) - CPU usage metrics
* [`cpufreq`](./cpufreqMetric.md) - CPU frequency data
* [`cpufreq_cpuinfo`](./cpufreqCpuinfoMetric.md) - CPU frequency from cpuinfo
* [`likwid`](./likwidMetric.md) - Hardware performance via Likwid
* [`topprocs`](./topprocsMetric.md) - Top process details

**Memory**
* [`memstat`](./memstatMetric.md) - Memory usage data
* [`numastats`](./numastatsMetric.md) - NUMA memory allocations

**Disk & I/O**
* [`diskstat`](./diskstatMetric.md) - Disk usage metrics
* [`iostat`](./iostatMetric.md) - I/O performance data

**Filesystems**
* [`beegfs_meta`](./beegfsmetaMetric.md) - BeeGFS metadata metrics
* [`beegfs_storage`](./beegfsstorageMetric.md) - BeeGFS storage metrics
* [`gpfs`](./gpfsMetric.md) - GPFS filesystem data
* [`lustrestat`](./lustreMetric.md) - Lustre filesystem metrics
* [`nfs3stat`](./nfs3Metric.md) - NFSv3 usage
* [`nfs4stat`](./nfs4Metric.md) - NFSv4 usage

**Network**
* [`ibstat`](./infinibandMetric.md) - InfiniBand network metrics
* [`netstat`](./netstatMetric.md) - Network interface data

**GPU & Accelerators**
* [`nvidia`](./nvidiaMetric.md) - NVIDIA GPU metrics
* [`rocm_smi`](./rocmsmiMetric.md) - AMD ROCm SMI data

**Hardware Monitoring**
* [`ipmistat`](./ipmiMetric.md) - IPMI sensor readings
* [`tempstat`](./tempMetric.md) - Temperature measurements

**Custom**
* [`customcmd`](./customCmdMetric.md) - Custom command output

## Todos

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
