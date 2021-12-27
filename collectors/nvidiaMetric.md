
## `nvidia` collector

```json
  "lustrestat": {
    "exclude_devices" : [
      "0","1"
    ],
    "exclude_metrics": [
      "fb_memory",
      "fan"
    ]
  }
```

Metrics:
* `util`
* `mem_util`
* `mem_total`
* `fb_memory`
* `temp`
* `fan`
* `ecc_mode`
* `perf_state`
* `power_usage_report`
* `graphics_clock_report`
* `sm_clock_report`
* `mem_clock_report`
* `max_graphics_clock`
* `max_sm_clock`
* `max_mem_clock`
* `ecc_db_error`
* `ecc_sb_error`
* `power_man_limit`
* `encoder_util`
* `decoder_util`

It uses a separate `type` in the metrics. The output metric looks like this:
`<name>,type=accelerator,type-id=<nvidia-gpu-id> value=<metric value> <timestamp>`

