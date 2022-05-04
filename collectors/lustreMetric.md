
## `lustrestat` collector

```json
  "lustrestat": {
    "lctl_command": "/path/to/lctl",
    "exclude_metrics": [
      "setattr",
      "getattr"
    ],
    "send_abs_values" : true,
    "send_derived_values" : true,
    "send_diff_values": true,
    "use_sudo": false
  }
```

The `lustrestat` collector uses the `lctl` application with the `get_param` option to get all `llite` metrics (Lustre client). The `llite` metrics are only available for root users. If password-less sudo is configured, you can enable `sudo` in the configuration.

Metrics:
* `lustre_read_bytes` (unit `bytes`)
* `lustre_read_requests` (unit `requests`)
* `lustre_write_bytes` (unit `bytes`)
* `lustre_write_requests` (unit `requests`)
* `lustre_open`
* `lustre_close`
* `lustre_getattr`
* `lustre_setattr`
* `lustre_statfs`
* `lustre_inode_permission`
* `lustre_read_bw` (if `send_derived_values == true`, unit `bytes/sec`)
* `lustre_write_bw` (if `send_derived_values == true`, unit `bytes/sec`)
* `lustre_read_requests_rate` (if `send_derived_values == true`, unit `requests/sec`)
* `lustre_write_requests_rate` (if `send_derived_values == true`, unit `requests/sec`)
* `lustre_read_bytes_diff` (if `send_diff_values == true`, unit `bytes`)
* `lustre_read_requests_diff` (if `send_diff_values == true`, unit `requests`)
* `lustre_write_bytes_diff` (if `send_diff_values == true`, unit `bytes`)
* `lustre_write_requests_diff` (if `send_diff_values == true`, unit `requests`)
* `lustre_open_diff` (if `send_diff_values == true`)
* `lustre_close_diff` (if `send_diff_values == true`)
* `lustre_getattr_diff` (if `send_diff_values == true`)
* `lustre_setattr_diff` (if `send_diff_values == true`)
* `lustre_statfs_diff` (if `send_diff_values == true`)
* `lustre_inode_permission_diff` (if `send_diff_values == true`)

This collector adds an `device` tag.