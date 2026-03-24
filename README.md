<!--
---
title: cc-metric-collector
description: Metric collecting node agent
categories: [cc-metric-collector]
tags: ['Admin']
weight: 2
hugo_path: docs/reference/cc-metric-collector/_index.md
---
-->

# cc-metric-collector

A node agent for measuring, processing and forwarding node level metrics. It is part of the [ClusterCockpit ecosystem](https://clustercockpit.org/docs/overview/). 

The `cc-metric-collector` sends (and maybe receives) metrics in the [InfluxDB line protocol](https://docs.influxdata.com/influxdb/cloud/reference/syntax/line-protocol/) as it provides flexibility while providing a separation between tags (like index columns in relational databases) and fields (like data columns). The `cc-metric-collector` consists of 4 components: collectors, router, sinks and receivers. The collectors read data from the current system and submit metrics to the router. The router can be configured to manipulate the metrics before forwarding them to the sinks. The receivers are also attached to the router like the collectors but they receive data from external source like other `cc-metric-collector` instances.


[![DOI](https://zenodo.org/badge/DOI/10.5281/zenodo.7438287.svg)](https://doi.org/10.5281/zenodo.7438287)


# Configuration

Configuration is implemented using a single json document that is distributed over network and may be persisted as file.
Supported metrics are documented [here](https://github.com/ClusterCockpit/cc-specifications/blob/master/interfaces/lineprotocol/README.md).

There is a main configuration file with basic settings that point to the other configuration files for the different components.

``` json
{
  "sinks-file": "sinks.json",
  "collectors-file" : "collectors.json",
  "receivers-file" : "receivers.json",
  "router-file" : "router.json",
  "main": {
    "interval": "10s",
    "duration": "1s"
  }
}
```

The `interval` defines how often the metrics should be read and send to the sink(s). The `duration` tells the collectors how long one measurement has to take. This is important for some collectors, like the `likwid` collector. For more information, see [here](./docs/configuration.md).

See the component READMEs for their configuration:

* [`collectors`](./collectors/README.md)
* [`sinks`](https://github.com/ClusterCockpit/cc-lib/blob/main/sinks/README.md)
* [`receivers`](https://github.com/ClusterCockpit/cc-lib/blob/main/receivers/README.md)
* [`router`](./internal/metricRouter/README.md)

# Installation

```
$ git clone git@github.com:ClusterCockpit/cc-metric-collector.git
$ make (downloads LIKWID, builds it as static library with 'direct' accessmode and copies all required files for the collector)
$ go get
$ make
```

For more information, see [here](./docs/building.md).

# Running

```
$ ./cc-metric-collector --help
Usage of ./cc-metric-collector:
  -config string
    	Path to configuration file (default "./config.json")
  -log string
    	Path for logfile (default "stderr")
  -loglevel string
    	Set log level (default "info")
  -once
    	Run all collectors only once
```

# Scenarios

The metric collector was designed with flexibility in mind, so it can be used in many scenarios. Here are a few:

```mermaid
flowchart TD
  subgraph a ["Cluster A"]
  nodeA[NodeA with CC collector]
  nodeB[NodeB with CC collector]
  nodeC[NodeC with CC collector]
  end
  a --> db[(Database)]
  db <--> ccweb("Webfrontend")
```

``` mermaid
flowchart TD
  subgraph a [ClusterA]
  direction LR
  nodeA[NodeA with CC collector]
  nodeB[NodeB with CC collector]
  nodeC[NodeC with CC collector]
  end
  subgraph b [ClusterB]
  direction LR
  nodeD[NodeD with CC collector]
  nodeE[NodeE with CC collector]
  nodeF[NodeF with CC collector]
  end
  a --> ccrecv{"CC collector as receiver"}
  b --> ccrecv
  ccrecv --> db[("Database1")]
  ccrecv -.-> db2[("Database2")]
  db <-.-> ccweb("Webfrontend")
```

# Contributing

The ClusterCockpit ecosystem is designed to be used by different HPC computing centers. Since configurations and setups differ between the centers, the centers likely have to put some work into `cc-metric-collector` to gather all desired metrics.

You are free to open an issue to request a collector but we would also be happy about PRs.

# Contact

* [Matrix.org ClusterCockpit General chat](https://matrix.to/#/#clustercockpit-dev:matrix.org)
* [Matrix.org ClusterCockpit Development chat](https://matrix.to/#/#clustercockpit:matrix.org)
