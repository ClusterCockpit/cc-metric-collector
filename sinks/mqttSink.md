## `mqtt` sink

The `mqtt` sink publishes all metrics into a MQTT network.

### Configuration structure

```json
{
  "<name>": {
    "type": "mqtt",
    "client_id" : "myid",
    "persistence_directory": "/tmp",
    "batch_size": 1000,
    "flush_delay": "1s",
    "dial_protocol": "tcp",
    "host": "dbhost.example.com",
    "port": 1883,
    "user": "exampleuser",
    "password" : "examplepw",
    "pause_timeout": "1s",
    "keep_alive_seconds": 10,
    "meta_as_tags" : [],
  }
}
```

- `type`: makes the sink an `mqtt` sink
- `client_id`: MQTT clients use a client_id to talk to the broker
- `persistence_directory`: MQTT stores messages temporarly on disc if the broker is not available. Folder needs to be writable (default: `/tmp`)
- `pause_timeout`: Waittime when published failed
- `keep_alive_seconds`: Keep the connection alive for some time. Recommended to be longer than global `interval`.
- `flush_delay`: Group metrics coming in to a single batch
- `batch_size`: Maximal batch size. If `batch_size` is reached before the end of `flush_delay`, the metrics are sent without further delay
- `dial_protocol`: Use `tcp` or `udp` for the MQTT communication
- `host`: Hostname of the MQTT broker
- `port`: Port number of the MQTT broker
- `user`: Username for authentication
- `password`: Password for authentication
- `meta_as_tags`: print all meta information as tags in the output (optional)
