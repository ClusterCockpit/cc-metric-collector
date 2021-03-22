# cc-metric-collector
A node agent for measuring, processing and forwarding node level metrics.

Open questions:

* Are hostname unique with a computing center or is it required to store the cluster name in addition to the hostname?
* What about memory domain granularity?

# Configuration

Configuration is implemented using a single json document that is distributed over network and may be persisted as file.
Granularity can be either `node`, or `core`. Frequency can be set on a per measurement basis.
Supported metrics are documented [here](https://github.com/ClusterCockpit/cc-specifications/blob/master/metrics/lineprotocol.md).

``` json
{
    "sink": {
        "user": "admin",
        "password": "12345",
        "host": "localhost",
        "port": "8080"
    },
    "interval": 5,
    "collectors": [
        "memstat",
        "loadstat",
        "netstat"
    ]
}
```
