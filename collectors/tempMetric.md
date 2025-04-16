<!--
---
title: Temperature metric collector
description: Collect thermal metrics from `/sys/class/hwmon/*`
categories: [cc-metric-collector]
tags: ['Admin']
weight: 2
hugo_path: docs/reference/cc-metric-collector/collectors/temp.md
---
-->


## `tempstat` collector

```json
  "tempstat": {
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
