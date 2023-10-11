## `http` receiver

The `http` receiver can be used receive metrics through HTTP POST requests.

### Configuration structure

```json
{
  "<name>": {
    "type": "http",
    "address" : "",
    "port" : "8080",
    "path" : "/write",
    "idle_timeout": "120s",
    "username": "myUser",
    "password": "myPW"
  }
}
```

- `type`: makes the receiver a `http` receiver
- `address`: Listen address
- `port`: Listen port
- `path`: URL path for the write endpoint
- `idle_timeout`: Maximum amount of time to wait for the next request when keep-alives are enabled should be larger than the measurement interval to keep the connection open
- `keep_alives_enabled`: Controls whether HTTP keep-alives are enabled. By default, keep-alives are enabled.
- `username`: username for basic authentication
- `password`: password for basic authentication

The HTTP endpoint listens to `http://<address>:<port>/<path>`

### Debugging

- Install [curl](https://curl.se/)
- Use curl to send message to `http` receiver

  ```bash
  curl http://localhost:8080/write \
  --user "myUser:myPW" \
  --data \
  "myMetric,hostname=myHost,type=hwthread,type-id=0,unit=Hz value=400000i 1694777161164284635
  myMetric,hostname=myHost,type=hwthread,type-id=1,unit=Hz value=400001i 1694777161164284635"
  ```
