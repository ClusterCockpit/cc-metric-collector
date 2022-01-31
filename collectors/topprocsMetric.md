
## `topprocs` collector

```json
  "topprocs": {
    "num_procs": 5
  }
```

The `topprocs` collector reads the TopX processes (sorted by CPU utilization, `ps -Ao comm --sort=-pcpu`). 

In contrast to most other collectors, the metric value is a `string`.



