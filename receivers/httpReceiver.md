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

### Debugging

- Install [curl](https://curl.se/)
- Use curl to send message to `http` receiver

  ```bash
  curl http://localhost:8080/write --data \
  "myMetric,hostname=myHost,type=hwthread,type-id=0,unit=Hz value=400000i 1694777161164284635
  myMetric,hostname=myHost,type=hwthread,type-id=1,unit=Hz value=400001i 1694777161164284635"
  ```
