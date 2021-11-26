# cc-metric-collector
A node agent for measuring, processing and forwarding node level metrics. It is part of the ClusterCockpit ecosystem.

The metric collector sends (and receives) metric in the [InfluxDB line protocol](https://docs.influxdata.com/influxdb/cloud/reference/syntax/line-protocol/) as it provides flexibility while providing a separation between tags (like index columns in relational databases) and fields (like data columns).

There is a single timer loop that triggers all collectors serially, collects the collectors' data and sends the metrics to the sink. This is done as all data is submitted with a single time stamp. The sinks currently use mostly blocking APIs.

The receiver runs as a go routine side-by-side with the timer loop and asynchronously forwards received metrics to the sink.

# Configuration

Configuration is implemented using a single json document that is distributed over network and may be persisted as file.
Supported metrics are documented [here](https://github.com/ClusterCockpit/cc-specifications/blob/master/metrics/lineprotocol_alternative.md).

``` json
{
  "interval": 3,
  "duration": 1,
  "collectors": [
    "memstat",
    "likwid",
    "loadavg",
    "netstat",
    "ibstat",
    "lustrestat",
    "topprocs",
    "cpustat",
    "nvidia"
  ],
  "sink": {
    "user": "admin",
    "password": "12345",
    "host": "localhost",
    "port": "8080",
    "database": "testdb",
    "organisation": "testorg",
    "type": "stdout"
  },
  "default_tags": {
    "cluster": "testcluster"
  },
  "receiver": {
    "type": "none",
    "address": "127.0.0.1",
    "port": "4222",
    "database": "testdb"
  },
  "collect_config": {
    "tempstat": {
      "tag_override": {
        "hwmon0": {
          "type": "socket",
          "type-id": "0"
        },
        "hwmon1": {
          "type": "socket",
          "type-id": "1"
        }
      }
    },
    "diskstat": {
      "exclude_metrics": [
        "read_ms"
      ]
    }
  }
}
```

The `interval` defines how often the metrics should be read and send to the sink. The `duration` tells collectors how long one measurement has to take. An example for this is the `likwid` collector which starts the hardware performance counter, waits for `duration` seconds and stops the counters again. If you configure a collector to do two measurments, the `duration` must be at least half the `interval`.

The `collectors` contains all collectors executed collectors. Each collector can be configured in the `collect_config` section. A more detailed list of all collectors and their configuration options can be found in the [README for collectors](./collectors/README.md).

The `sink` section contains the configuration where the data should be transmitted to. There are currently four sinks supported `influxdb`, `nats`, `http` and `stdout`. See [README for sinks](./sinks/README.md) for more information about the individual sinks and which configuration field they are using.

In the `default_tags` section, one can define key-value-pairs (only strings) that are added to each sent out metric. This can be useful for cluster names like in the example JSON or information like rank or island for orientation.

With `receiver`, the collector can be used as a router by receiving metrics and forwarding them to the configured sink. There are currently only types `none` (for no receiver) and `nats`. For more information see the [README in receivers](./receivers/README.md).

# Installation

```
$ git clone git@github.com:ClusterCockpit/cc-metric-collector.git
$ cd cc-metric-collector/collectors
$ edit Makefile (for LIKWID collector)
$ make (downloads LIKWID, builds it as static library and copies all required files for the collector. Uses sudo in case of own accessdaemon)
$ cd ..
$ go get (requires at least golang 1.13)
$ go build metric-collector
```

# Running

```
$ ./metric-collector --help
Usage of metric-collector:
  -config string
    	Path to configuration file (default "./config.json")
  -log string
    	Path for logfile (default "stderr")
  -once
    	Run all collectors only once
  -pidfile string
    	Path for PID file (default "/var/run/cc-metric-collector.pid")
```

# Todos

- [ ] Use only non-blocking APIs for the sinks
- [x] Collector specific configuration in global JSON file? Changing the configuration inside the Go code is not user-friendly.
- [ ] Mark collectors as 'can-run-in-parallel' and use goroutines for them. There are only a few collectors that should run serially (e.g. LIKWID)
- [ ] Configuration option for receivers to add other tags. Additonal flag to tell whether default tags should be added as well.
- [ ] CLI option to get help output for collectors, sinks and receivers about their configuration options and metrics

# Contributing
The ClusterCockpit ecosystem is designed to be used by different HPC computing centers. Since configurations and setups differ between the centers, the centers likely have to put some work into the cc-metric-collector to gather all desired metrics.

You are free to open an issue to request a collector but we would also be happy about PRs.

# Contact 

[Matrix.org ClusterCockpit General chat](https://matrix.to/#/#clustercockpit-dev:matrix.org)
[Matrix.org ClusterCockpit Development chat](https://matrix.to/#/#clustercockpit:matrix.org)
