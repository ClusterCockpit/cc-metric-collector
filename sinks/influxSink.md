## `influxdb` sink

The `influxdb` sink uses the official [InfluxDB golang client](https://pkg.go.dev/github.com/influxdata/influxdb-client-go/v2) to write the metrics to an InfluxDB database in a **blocking** fashion. It provides only support for V2 write endpoints (InfluxDB 1.8.0 or later).

### Configuration structure

```json
{
  "<name>": {
    "type": "influxdb",
    "database" : "mymetrics",
    "host": "dbhost.example.com",
    "port": "4222",
    "user": "exampleuser",
    "password" : "examplepw",
    "organization": "myorg",
    "ssl": true,
    "flush_delay" : "1s",
    "batch_size" : 1000,
    "use_gzip": true,
    "precision": "s",
    "process_messages" : {
      "see" : "docs of message processor for valid fields"
    },
    "meta_as_tags" : []
  }
}
```

- `type`: makes the sink an `influxdb` sink
- `database`: All metrics are written to this bucket
- `host`: Hostname of the InfluxDB database server
- `port`: Port number (as string) of the InfluxDB database server
- `user`: Username for basic authentication
- `password`: Password for basic authentication
- `organization`: Organization in the InfluxDB
- `ssl`: Use SSL connection
- `flush_delay`: Group metrics coming in to a single batch
- `batch_size`: Maximal batch size. If `batch_size` is reached before the end of `flush_delay`, the metrics are sent without further delay
- `precision`: Precision of the timestamp. Valid values are 's', 'ms', 'us' and 'ns'. (default is 's')
- `process_messages`: Process messages with given rules before progressing or dropping, see [here](../pkg/messageProcessor/README.md) (optional)
- `meta_as_tags`: print all meta information as tags in the output (deprecated, optional)

Influx client options:
=======
- `batch_size`: Maximal batch size
- `meta_as_tags`: move meta information keys to tags (optional)
- `http_request_timeout`: HTTP request timeout
- `retry_interval`: retry interval
- `max_retry_interval`: maximum delay between each retry attempt
- `retry_exponential_base`: base for the exponential retry delay
- `max_retries`: maximum count of retry attempts of failed writes
- `max_retry_time`: maximum total retry timeout
- `use_gzip`: Specify whether to use GZip compression in write requests

### Using `influxdb` sink for communication with cc-metric-store

The cc-metric-store only accepts metrics with a timestamp precision in seconds, so it is required to use `"precision": "s"`.