## `http` sink

The `http` sink uses POST requests to a HTTP server to submit the metrics in the InfluxDB line-protocol format. It uses JSON web tokens for authentification. The sink creates batches of metrics before sending, to reduce the HTTP traffic.

### Configuration structure

```json
{
  "<name>": {
    "type": "http",
    "meta_as_tags" : true,
    "url" : "https://my-monitoring.example.com:1234/api/write",
    "jwt" : "blabla.blabla.blabla",
    "timeout": "5s",
    "max_idle_connections" : 10,
    "idle_connection_timeout" : "5s",
    "flush_delay": "2s",
    "batch_size" : 100
  }
}
```

- `type`: makes the sink an `http` sink
- `meta_as_tags`: print all meta information as tags in the output (optional)
- `url`: The full URL of the endpoint
- `jwt`: JSON web tokens for authentification (Using the *Bearer* scheme)
- `timeout`: General timeout for the HTTP client (default '5s')
- `max_idle_connections`: Maximally idle connections (default 10)
- `idle_connection_timeout`: Timeout for idle connections (default '5s')
- `flush_delay`: Batch all writes arriving in during this duration (default '1s', batching can be disabled by setting it to 0)
- `batch_size`: Maximal number of batched metrics. Either it is flushed because batch size or the `flush_delay` is reached
