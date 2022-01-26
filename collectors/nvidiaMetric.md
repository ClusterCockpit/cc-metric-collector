
## `nvidia` collector

```json
  "nvidia": {
    "exclude_devices" : [
      "0","1"
    ],
    "exclude_metrics": [
      "nv_fb_memory",
      "nv_fan"
    ]
  }
```

Metrics:
* `nv_util`
* `nv_mem_util`
* `nv_mem_total`
* `nv_fb_memory`
* `nv_temp`
* `nv_fan`
* `nv_ecc_mode`
* `nv_perf_state`
* `nv_power_usage_report`
* `nv_graphics_clock_report`
* `nv_sm_clock_report`
* `nv_mem_clock_report`
* `nv_max_graphics_clock`
* `nv_max_sm_clock`
* `nv_max_mem_clock`
* `nv_ecc_db_error`
* `nv_ecc_sb_error`
* `nv_power_man_limit`
* `nv_encoder_util`
* `nv_decoder_util`

It uses a separate `type` in the metrics. The output metric looks like this:
`<name>,type=accelerator,type-id=<nvidia-gpu-id> value=<metric value> <timestamp>`

