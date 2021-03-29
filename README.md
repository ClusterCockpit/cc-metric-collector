# cc-metric-collector
A node agent for measuring, processing and forwarding node level metrics.

Open questions:

* Are hostname unique with a computing center or is it required to store the cluster name in addition to the hostname?
* What about memory domain granularity?

# Configuration

Configuration is implemented using a single json document that is distributed over network and may be persisted as file.
Supported metrics are documented [here](https://github.com/ClusterCockpit/cc-specifications/blob/master/metrics/lineprotocol.md).

``` json
{
    "sink": {
        "user": "admin",
        "password": "12345",
        "host": "localhost",
        "port": "8080",
        "database": "testdb",
        "type": "stdout"
    },
    "interval" : 3,
    "duration" : 1,
    "collectors": [
        "memstat",
        "likwid",
        "loadavg",
        "netstat",
        "ibstat",
        "lustrestat"
    ]
}
```

All available collectors are listed in the above JSON. There are currently three sinks supported `influxdb`, `nats` and `stdout`. The `interval` defines how often the metrics should be read and send to the sink. The `duration` tells collectors how long one measurement has to take. An example for this is the `likwid` collector which starts the hardware performance counter, waits for `duration` seconds and stops the counters again. For most systems, the `likwid` collector has to do two measurements, thus `interval` must be larger than two times `duration`.


