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
   "sink": "db.monitoring.center.de",
   "report": {
      levels: ["core","node"],
      interval: 120
      },
   "schedule": {
      "core": {
         "frequency": 30,
         "duration": 10},
      "node":{
         "frequency": 60,
         "duration": 20}
   },
   "metrics": [
      "ipc",
      "flops_any",
      "clock",
      "load",
      "mem_bw",
      "mem_used",
      "net_bw",
      "file_bw"
   ]
}
```
