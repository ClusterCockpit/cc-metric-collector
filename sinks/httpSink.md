## `http` sink

The `http` sink uses POST requests to a HTTP server to submit the metrics in the InfluxDB line-protocol format. It uses JSON web tokens for authentification. The sink creates batches of metrics before sending, to reduce the HTTP traffic.

### Configuration structure

```json
{
  "<name>": {
    "type": "http",
    "url" : "https://my-monitoring.example.com:1234/api/write",
    "jwt" : "blabla.blabla.blabla",
    "username": "myUser",
    "password": "myPW",
    "timeout": "5s",
    "idle_connection_timeout" : "5s",
    "flush_delay": "2s",
    "batch_size": 1000,
    "precision": "s",
    "process_messages" : {
      "see" : "docs of message processor for valid fields"
    },
    "meta_as_tags" : []
  }
}
```

- `type`: makes the sink an `http` sink
- `url`: The full URL of the endpoint
- `jwt`: JSON web tokens for authentication (Using the *Bearer* scheme)
- `username`: username for basic authentication
- `password`: password for basic authentication
- `timeout`: General timeout for the HTTP client (default '5s')
- `max_retries`: Maximum number of retries to connect to the http server
- `idle_connection_timeout`: Timeout for idle connections (default '120s'). Should be larger than the measurement interval to keep the connection open
- `flush_delay`: Batch all writes arriving in during this duration (default '1s', batching can be disabled by setting it to 0)
- `batch_size`: Maximal batch size. If `batch_size` is reached before the end of `flush_delay`, the metrics are sent without further delay
- `precision`: Precision of the timestamp. Valid values are 's', 'ms', 'us' and 'ns'. (default is 's')
- `process_messages`: Process messages with given rules before progressing or dropping, see [here](../pkg/messageProcessor/README.md) (optional)
- `meta_as_tags`: print all meta information as tags in the output (deprecated, optional)

### Using `http` sink for communication with cc-metric-store

The cc-metric-store only accepts metrics with a timestamp precision in seconds, so it is required to use `"precision": "s"`.