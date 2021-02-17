# cc-metric-collector
A node agent for measuring, processing and forwarding node level metrics.

Open questions:

* Are hostname unique with a computing center or is it required to store the cluster name in addition to the hostname?
* What about memory domain granularity?

# Configuration

Configuration is implemented using a single json document that is distributed over network.

``` json
{
"sink": "db.monitoring.center.de",
"granularity": "core",
"frequency": {
"core": 30,
"node": 60
}
metrics: [
ipc,
flops_any,
clock,
load,
mem_bw,
mem_used,
net_bw,
file_bw
]}
```
