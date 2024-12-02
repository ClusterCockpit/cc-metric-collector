
## `lustre_jobstat` collector

**Note**: This collector is meant to run on the Lustre servers, **not** the clients

The Lustre filesystem provides a feature (`job_stats`) to group processes on client side with an identifier string (like a compute job with its jobid) and retrieve the file system operation counts on the server side. Check the section [How to configure `job_stats`]() for more information.

### Configuration

```json
  "lustre_jobstat_": {
    "lctl_command": "/path/to/lctl",
    "use_sudo": false,
    "exclude_metrics": [
      "setattr",
      "getattr"
    ],
    "send_abs_values" : true,
    
    "jobid_regex" : "^(?P<jobid>[%d%w%.]+)$"
  }
```

The `lustre_jobstat` collector uses the `lctl` application with the `get_param` option to get all `mdt.*.job_stats` and `obdfilter.*.job_stats` metrics. These metrics are only available for root users. If password-less sudo is configured, you can enable `sudo` in the configuration. In the `exclude_metrics` list, some metrics can be excluded to reduce network traffic and storage. With the `send_abs_values` flag, the collector sends absolute values for the configured metrics. The `jobid_regex` can be used to split the Lustre `job_stats` identifier into multiple parts. Since JSON cannot handle strings like `\d`, use `%` instead of `\`.

Metrics:
- `lustre_job_read_samples` (unit: `requests`)
- `lustre_job_read_min_bytes` (unit: `bytes`)
- `lustre_job_read_max_bytes` (unit: `bytes`)

The collector adds the tags: `type=jobid,typeid=<jobid_from_regex>,stype=device,stype=<device_name_from_output>`.

The collector adds the mega information: `unit=<unit>,scope=job`

### How to configure `job_stats`
