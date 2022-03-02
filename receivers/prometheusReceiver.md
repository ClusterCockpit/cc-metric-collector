## `prometheus` receiver

The `prometheus` receiver can be used to scrape the metrics of a single `prometheus` client. It does **not** use any official Golang library but making simple HTTP get requests and parse the response.

### Configuration structure

```json
{
  "<name>": {
    "type": "prometheus",
    "address" : "testpromhost",
    "port" : "12345",
    "path" : "/prometheus",
    "interval": "5s",
    "ssl" : true,
  }
}
```

- `type`: makes the receiver a `prometheus` receiver
- `address`: Hostname or IP of the Prometheus agent
- `port`: Port of Prometheus agent
- `path`: Path to the Prometheus endpoint
- `interval`: Scrape the Prometheus endpoint in this interval (default '5s')
- `ssl`: Use SSL or not

The receiver requests data from `http(s)://<address>:<port>/<path>`.