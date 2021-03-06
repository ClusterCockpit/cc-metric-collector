# cc-metric-collector
A node agent for measuring, processing and forwarding node level metrics part of the ClusterCockpit ecosystem.

# Configuration

Configuration is implemented using a single json document that is distributed over network and may be persisted as file.
Supported metrics are documented [here](https://github.com/ClusterCockpit/cc-specifications/blob/master/metrics/lineprotocol.md).

``` json
{
    "interval" : 3,
    "duration" : 1,
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
    }
}
```

The `interval` defines how often the metrics should be read and send to the sink. The `duration` tells collectors how long one measurement has to take. An example for this is the `likwid` collector which starts the hardware performance counter, waits for `duration` seconds and stops the counters again. For most systems, the `likwid` collector has to do two measurements, thus `interval` must be larger than two times `duration`.

All available collectors are listed in the above JSON. A more detailed list can be found in the [README for collectors](./collectors/README.md).

There are currently three sinks supported `influxdb`, `nats` and `stdout`. See [README for sinks](./sinks/README.md).

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
```

# Internals
The metric collector sends (and receives) metric in the [InfluxDB line protocol](https://docs.influxdata.com/influxdb/cloud/reference/syntax/line-protocol/) as it provides flexibility while providing a separation between tags (like index columns in relational databases) and fields (like data columns).

There is a single timer loop that triggers all collectors serially, collects the collectors' data and sends the metrics to the sink. This is done as all data is submitted with a single time stamp. The sinks currently use mostly blocking APIs.

The receiver runs as a go routine side-by-side with the timer loop and asynchronously forwards received metrics to the sink.

# Todos

- [ ] Use only non-blocking APIs for the sinks
- [ ] Collector specific configuration in global JSON file? Changing the configuration inside the Go code is not user-friendly.
- [ ] Metrics in memory domain granularity?

