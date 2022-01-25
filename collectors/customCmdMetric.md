
## `customcmd` collector

```json
  "customcmd": {
    "exclude_metrics": [
      "mymetric"
    ],
    "files" : [
      "/var/run/myapp.metrics"
    ],
    "commands" : [
      "/usr/local/bin/getmetrics.pl"
    ]
  }
```

The `customcmd` collector reads data from files and the output of executed commands. The files and commands can output multiple metrics (separated by newline) but the have to be in the [InfluxDB line protocol](https://docs.influxdata.com/influxdb/cloud/reference/syntax/line-protocol/). If a metric is not parsable, it is skipped. If a metric is not required, it can be excluded from forwarding it to the sink.


