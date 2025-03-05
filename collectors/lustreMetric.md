## `lustrestat` collector

```json
  "lustrestat": {
    "lctl_command": "/path/to/lctl",
    "exclude_metrics": [
      "lustre_setattr",
      "lustre_getattr"
    ],
    "only_metrics": [
      "lustre_read_bytes",
      "lustre_read_bytes_diff",
      "lustre_read_bw",
      "lustre_open",
      "lustre_open_diff"
    ],
    "send_abs_values": true,
    "send_diff_values": true,
    "send_derived_values": true,
    "use_sudo": false
  }
```

The `lustrestat` collector uses the `lctl` application with the `get_param` option to get all `llite` metrics (Lustre client). The `llite` metrics are only available for root users. If password-less sudo is configured, you can enable `sudo` in the configuration.

At least one of the settings for absolute, diff, and derived values must be set to true.

Both filtering mechanisms are supported:
- `exclude_metrics`: Excludes the specified metrics.
- `only_metrics`: If provided, only the listed metrics are collected. This takes precedence over `exclude_metrics`.


Metrics are categorized as follows:

**Absolute Metrics:**
- `lustre_read_bytes` (unit: `bytes`)
- `lustre_read_requests` (unit: `requests`)
- `lustre_write_bytes` (unit: `bytes`)
- `lustre_write_requests` (unit: `requests`)
- `lustre_open`
- `lustre_close`
- `lustre_getattr`
- `lustre_setattr`
- `lustre_statfs`
- `lustre_inode_permission`

**Diff Metrics:**
- `lustre_read_bytes_diff` (unit: `bytes`)
- `lustre_read_requests_diff` (unit: `requests`)
- `lustre_write_bytes_diff` (unit: `bytes`)
- `lustre_write_requests_diff` (unit: `requests`)
- `lustre_open_diff`
- `lustre_close_diff`
- `lustre_getattr_diff`
- `lustre_setattr_diff`
- `lustre_statfs_diff`
- `lustre_inode_permission_diff`

**Derived Metrics:**
- `lustre_read_bw` (unit: `bytes/sec`)
- `lustre_write_bw` (unit: `bytes/sec`)
- `lustre_read_requests_rate` (unit: `requests/sec`)
- `lustre_write_requests_rate` (unit: `requests/sec`)

This collector adds a `device` tag.
