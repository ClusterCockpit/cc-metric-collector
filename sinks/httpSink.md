## `http` sink

The `http` sink uses POST requests to a HTTP server to submit the metrics in the InfluxDB line-protocol format. It uses JSON web tokens for authentification. The sink creates batches of metrics before sending, to reduce the HTTP traffic.

### Configuration structure

```json
{
  "<name>": {
    "type": "http",
    "meta_as_tags" : true,
    "database" : "mymetrics",
    "host": "dbhost.example.com",
    "port": "4222",
    "jwt" : "0x0000q231",
    "ssl" : false
  }
}
```

- `type`: makes the sink an `http` sink
- `meta_as_tags`: print all meta information as tags in the output (optional)
- `database`: All metrics are written to this bucket 
- `host`: Hostname of the InfluxDB database server
- `port`: Portnumber (as string) of the InfluxDB database server
- `jwt`: JSON web tokens for authentification
- `ssl`: Activate SSL encryption