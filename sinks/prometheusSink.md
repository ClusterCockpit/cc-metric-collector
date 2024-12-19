## `prometheus` sink

The `prometheus` sink publishes all metrics via an HTTP server ready to be scraped by a [Prometheus](https://prometheus.io) server. It creates gauge metrics for all node metrics and gauge vectors for all metrics with a subtype like 'device', 'cpu' or 'socket'. 


### Configuration structure

```json
{
  "<name>": {
    "type": "prometheus",
    "host": "localhost",
    "port": "8080",
    "path": "metrics",
    "process_messages" : {
      "see" : "docs of message processor for valid fields"
    },
    "meta_as_tags" : []
  }
}
```

- `type`: makes the sink an `prometheus` sink
- `host`: The HTTP server gets bound to that IP/hostname
- `port`: Portnumber (as string) for the HTTP server
- `path`: Path where the metrics should be servered. The metrics will be published at `host`:`port`/`path`
- `group_as_namespace`: Most metrics contain a group as meta information like 'memory', 'load'. With this the metric names are extended to `group`_`name` if possible.
- `process_messages`: Process messages with given rules before progressing or dropping, see [here](../pkg/messageProcessor/README.md) (optional)
- `meta_as_tags`: print all meta information as tags in the output (deprecated, optional)