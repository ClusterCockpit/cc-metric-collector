## `http` receiver

The `http` receiver can be used receive metrics through HTTP POST requests.

### Configuration structure

```json
{
  "<name>": {
    "type": "http",
    "address" : "",
    "port" : "8080",
    "path" : "/write"
  }
}
```

- `type`: makes the receiver a `http` receiver
- `address`: Listen address
- `port`: Listen port
- `path`: URL path for the write endpoint

The HTTP endpoint listens to `http://<address>:<port>/<path>`
