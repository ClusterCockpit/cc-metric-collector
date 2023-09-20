## `http` sink

The `http` sink uses POST requests to a HTTP server to submit the metrics in the InfluxDB line-protocol format. It uses JSON web tokens for authentification. The sink creates batches of metrics before sending, to reduce the HTTP traffic.

### Configuration structure

```json
{
  "<name>": {
    "type": "http",
    "meta_as_tags" : [
      "meta-key"
    ],
    "url" : "https://my-monitoring.example.com:1234/api/write",
    "jwt" : "blabla.blabla.blabla",
    "username": "myUser",
    "password": "myPW",
    "timeout": "5s",
    "idle_connection_timeout" : "5s",
    "flush_delay": "2s",
  }
}
```

- `type`: makes the sink an `http` sink
- `meta_as_tags`: Move specific meta information to the tags in the output (optional)
- `url`: The full URL of the endpoint
- `jwt`: JSON web tokens for authentication (Using the *Bearer* scheme)
- `username`: username for basic authentication
- `password`: password for basic authentication
- `timeout`: General timeout for the HTTP client (default '5s')
- `max_retries`: Maximum number of retries to connect to the http server
- `idle_connection_timeout`: Timeout for idle connections (default '120s'). Should be larger than the measurement interval to keep the connection open
- `flush_delay`: Batch all writes arriving in during this duration (default '1s', batching can be disabled by setting it to 0)
