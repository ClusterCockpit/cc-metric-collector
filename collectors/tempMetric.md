
## `tempstat` collector

```json
  "lustrestat": {
    "tag_override" : {
        "<device like hwmon1>" : {
            "type" : "socket",
            "type-id" : "0"
        }
    },
    "exclude_metrics": [
      "metric1",
      "metric2"
    ]
  }
```

The `tempstat` collector reads the data from `/sys/class/hwmon/<device>/tempX_{input,label}`

Metrics:
* `temp_*`: The metric name is taken from the `label` files.
