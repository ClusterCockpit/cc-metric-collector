## `nats` sink

The `nats` sink publishes all metrics into a NATS network. The publishing key is the database name provided in the configuration file

### Configuration structure

```json
{
  "<name>": {
    "type": "nats",
    "meta_as_tags" : true,
    "database" : "mymetrics",
    "host": "dbhost.example.com",
    "port": "4222",
    "user": "exampleuser",
    "password" : "examplepw"
  }
}
```

- `type`: makes the sink an `nats` sink
- `meta_as_tags`: print all meta information as tags in the output (optional)
- `database`: All metrics are published with this subject
- `host`: Hostname of the NATS server
- `port`: Port number (as string) of the NATS server
- `user`: Username for basic authentication
- `password`: Password for basic authentication
