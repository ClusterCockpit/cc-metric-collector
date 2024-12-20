## `nats` sink

The `nats` sink publishes all metrics into a NATS network. The publishing key is the database name provided in the configuration file

### Configuration structure

```json
{
  "<name>": {
    "type": "nats",
    "database" : "mymetrics",
    "host": "dbhost.example.com",
    "port": "4222",
    "user": "exampleuser",
    "password" : "examplepw",
    "nkey_file": "/path/to/nkey_file",
    "flush_delay": "10s",
    "precision": "s",
    "process_messages" : {
      "see" : "docs of message processor for valid fields"
    },
    "meta_as_tags" : []
  }
}
```

- `type`: makes the sink an `nats` sink
- `database`: All metrics are published with this subject
- `host`: Hostname of the NATS server
- `port`: Port number (as string) of the NATS server
- `user`: Username for basic authentication
- `password`: Password for basic authentication
- `nkey_file`: Path to credentials file with NKEY
- `flush_delay`: Maximum time until metrics are sent out (default '5s')
- `precision`: Precision of the timestamp. Valid values are 's', 'ms', 'us' and 'ns'. (default is 's')
- `process_messages`: Process messages with given rules before progressing or dropping, see [here](../pkg/messageProcessor/README.md)  (optional)
- `meta_as_tags`: print all meta information as tags in the output (deprecated, optional)

### Using `nats` sink for communication with cc-metric-store

The cc-metric-store only accepts metrics with a timestamp precision in seconds, so it is required to use `"precision": "s"`.