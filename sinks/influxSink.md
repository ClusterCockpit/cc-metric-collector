## `influxdb` sink

The `influxdb` sink uses the official [InfluxDB golang client](https://pkg.go.dev/github.com/influxdata/influxdb-client-go/v2) to write the metrics to an InfluxDB database in a **blocking** fashion. It provides only support for V2 write endpoints (InfluxDB 1.8.0 or later).

### Configuration structure

```json
{
  "<name>": {
    "type": "influxdb",
    "meta_as_tags" : true,
    "database" : "mymetrics",
    "host": "dbhost.example.com",
    "port": "4222",
    "user": "exampleuser",
    "password" : "examplepw",
    "organization": "myorg",
    "ssl": true,
    "flush_delay" : "1s",
    "batch_size" : 1000,
    "use_gzip": true
  }
}
```

- `type`: makes the sink an `influxdb` sink
- `meta_as_tags`: print all meta information as tags in the output (optional)
- `database`: All metrics are written to this bucket
- `host`: Hostname of the InfluxDB database server
- `port`: Port number (as string) of the InfluxDB database server
- `user`: Username for basic authentication
- `password`: Password for basic authentication
- `organization`: Organization in the InfluxDB
- `ssl`: Use SSL connection
- `flush_delay`: Group metrics coming in to a single batch
- `batch_size`: Maximal batch size. If `batch_size` is reached before the end of `flush_delay`, the metrics are sent without further delay

Influx client options:

- `http_request_timeout`: HTTP request timeout
- `retry_interval`: retry interval
- `max_retry_interval`: maximum delay between each retry attempt
- `retry_exponential_base`: base for the exponential retry delay
- `max_retries`: maximum count of retry attempts of failed writes
- `max_retry_time`: maximum total retry timeout
- `use_gzip`: Specify whether to use GZip compression in write requests
