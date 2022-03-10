## `influxasync` sink

The `influxasync` sink uses the official [InfluxDB golang client](https://pkg.go.dev/github.com/influxdata/influxdb-client-go/v2) to write the metrics to an InfluxDB database in a **non-blocking** fashion. It provides only support for V2 write endpoints (InfluxDB 1.8.0 or later).


### Configuration structure

```json
{
  "<name>": {
    "type": "influxasync",
    "meta_as_tags" : true,
    "database" : "mymetrics",
    "host": "dbhost.example.com",
    "port": "4222",
    "user": "exampleuser",
    "password" : "examplepw",
    "organization": "myorg",
    "ssl": true,
    "batch_size": 200,
    "retry_interval" : "1s",
    "retry_exponential_base" : 2,
    "max_retries": 20,
    "max_retry_time" : "168h"
  }
}
```

- `type`: makes the sink an `influxdb` sink
- `meta_as_tags`: print all meta information as tags in the output (optional)
- `database`: All metrics are written to this bucket 
- `host`: Hostname of the InfluxDB database server
- `port`: Portnumber (as string) of the InfluxDB database server
- `user`: Username for basic authentification
- `password`: Password for basic authentification
- `organization`: Organization in the InfluxDB
- `ssl`: Use SSL connection
- `batch_size`: batch up metrics internally, default 100
- `retry_interval`: Base retry interval for failed write requests, default 1s
- `retry_exponential_base`: The retry interval is exponentially increased with this base, default 2
- `max_retries`: Maximal number of retry attempts
- `max_retry_time`: Maximal time to retry failed writes, default 168h (one week)

For information about the calculation of the retry interval settings, see [offical influxdb-client-go documentation](https://github.com/influxdata/influxdb-client-go#handling-of-failed-async-writes)