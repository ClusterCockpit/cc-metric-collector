## `libganglia` sink

The `libganglia` sink interacts directly with the library of the [Ganglia Monitoring System](http://ganglia.info/) to submit the metrics. Consequently, it needs to be installed on all nodes. But this is commonly the case if you want to use Ganglia, because it requires at least a node daemon (`gmond` or `ganglia-monitor`) to work.

The `libganglia` sink has probably less overhead compared to the `ganglia` sink because it does not require any process generation but initializes the environment and UDP connections only once.


### Configuration structure

```json
{
  "<name>": {
    "type": "libganglia",
    "gmetric_config" : "/path/to/gmetric/config",
    "cluster_name": "MyCluster",
    "add_ganglia_group" : true,
    "add_type_to_name": true,
    "add_units" : true,
    "process_messages" : {
      "see" : "docs of message processor for valid fields"
    },
    "meta_as_tags" : []
  }
}
```

- `type`: makes the sink an `libganglia` sink
- `gmond_config`: Path to the Ganglia configuration file `gmond.conf` (default: `/etc/ganglia/gmond.conf`)
- `cluster_name`: Set a cluster name for the metric. If not set, it is taken from `gmond_config`
- `add_ganglia_group`: Add a Ganglia metric group based on meta information. Some old versions of `gmetric` do not support the `--group` option
- `add_type_to_name`: Ganglia commonly uses only node-level metrics but with cc-metric-collector, there are metrics for cpus, memory domains, CPU sockets and the whole node. In order to get  eeng, this option prefixes the metric name with `<type><type-id>_` or `device_` depending on the metric tags and meta information. For metrics of the whole node `type=node`, no prefix is added
- `add_units`: Add metric value unit if there is a `unit` entry in the metric tags or meta information
- `process_messages`: Process messages with given rules before progressing or dropping, see [here](../pkg/messageProcessor/README.md)  (optional)
- `meta_as_tags`: print all meta information as tags in the output (deprecated, optional)

### Ganglia Installation

My development system is Ubuntu 20.04. To install the required libraries with `apt`:

```
$ sudo apt install libganglia1
```

The `libganglia.so` gets installed in `/usr/lib`. The Ganglia headers `libganglia1-dev` are **not** required.

I added a `Makefile` in the `sinks` subfolder that searches for the library in `/usr` and creates a symlink (`sinks/libganglia.so`) for running/building the cc-metric-collector. So just type `make` before running/building in the main folder or the `sinks` subfolder.