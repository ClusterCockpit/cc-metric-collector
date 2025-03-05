## `nvidia` collector

{
  "nvidia": {
    "exclude_devices": [
      "0",
      "1",
      "00000000:FF:01.0"
    ],
    "exclude_metrics": [
      "nv_fb_mem_used",
      "nv_fan"
    ],
    "only_metrics": [
      "nv_nvlink_ecc_errors_sum",
      "nv_nvlink_ecc_errors_sum_diff"
    ],
    "send_diff_values": true,
    "process_mig_devices": false,
    "use_pci_info_as_type_id": true,
    "add_pci_info_tag": false,
    "add_uuid_meta": false,
    "add_board_number_meta": false,
    "add_serial_meta": false,
    "use_uuid_for_mig_device": false,
    "use_slice_for_mig_device": false,
    "use_memory_info_v2": false
  }
}

The `nvidia` collector gathers metrics from NVIDIA GPUs using the NVIDIA Management Library (NVML). It can be configured to exclude specific devices with the `exclude_devices` option, which accepts device indices (e.g., `"0"`, `"1"`) as used by `nvmlDeviceGetHandleByIndex()` or PCI addresses in NVML format (e.g., `"%08X:%02X:%02X.0"`).

Both filtering mechanisms are supported:
- `exclude_metrics`: Excludes the specified metrics.
- `only_metrics`: If provided, only the listed metrics are collected. This takes precedence over `exclude_metrics`.

### Differential Metrics
- **`send_diff_values`**: When set to `true`, differential metrics (e.g., `nv_ecc_corrected_error_diff`, `nv_nvlink_ecc_errors_sum_diff`) are calculated and sent alongside absolute values. These represent the change since the last measurement.

### MIG Devices
By default, only physical GPUs are monitored. To include MIG (Multi-Instance GPU) devices, set `process_mig_devices` to `true`. This adds tags `stype=mig` and `stype-id=<mig_index>` to MIG-specific metrics. The `stype-id` can be customized:
- **`use_uuid_for_mig_device`**: Uses the MIG UUID (e.g., `MIG-6a9f7cc8-6d5b-5ce0-92de-750edc4d8849`).
- **`use_slice_for_mig_device`**: Uses the MIG slice name (e.g., `1g.5gb`).

### Tags and Metadata
Metrics use `type=accelerator` as a tag. The `type-id` defaults to the device handle index. Additional options include:
- **`use_pci_info_as_type_id`**: Uses the PCI ID (e.g., `00000000:FF:01.0`) as `type-id` instead of the index.
- **`add_pci_info_tag`**: Adds the PCI ID as a separate `pci_identifier` tag, while keeping the index as `type-id`.
- **`add_uuid_meta`**, **`add_board_number_meta`**, **`add_serial_meta`**: Add UUID, board part number, or serial number to the metadata (not sent to sinks unless configured otherwise).

### Memory Info
- **`use_memory_info_v2`**: When `true`, uses `nvmlDeviceGetMemoryInfo_v2` for more detailed memory metrics (i.e.  `mem_reserved` not part of `mem_used`). Defaults to `false`, falling back to `nvmlDeviceGetMemoryInfo`.

### Metrics
The following metrics are available. All `nv_nvlink_*` metrics are always delivered as `_sum` (aggregated across all NVLinks). If multiple devices are present, they are also provided as per-device metrics with `stype=nvlink` and `stype-id=<link_number>`.

#### Absolute Metrics
- `nv_util` (unit: `%`)
- `nv_mem_util` (unit: `%`)
- `nv_fb_mem_total` (unit: `MByte`)
- `nv_fb_mem_used` (unit: `MByte`)
- `nv_fb_mem_reserved` (unit: `MByte`)
- `nv_bar1_mem_total` (unit: `MByte`)
- `nv_bar1_mem_used` (unit: `MByte`)
- `nv_temp` (unit: `degC`)
- `nv_fan` (unit: `%`)
- `nv_ecc_mode`
- `nv_perf_state`
- `nv_power_usage` (unit: `watts`)
- `nv_graphics_clock` (unit: `MHz`)
- `nv_sm_clock` (unit: `MHz`)
- `nv_mem_clock` (unit: `MHz`)
- `nv_video_clock` (unit: `MHz`)
- `nv_max_graphics_clock` (unit: `MHz`)
- `nv_max_sm_clock` (unit: `MHz`)
- `nv_max_mem_clock` (unit: `MHz`)
- `nv_max_video_clock` (unit: `MHz`)
- `nv_ecc_uncorrected_error`
- `nv_ecc_corrected_error`
- `nv_power_max_limit` (unit: `watts`)
- `nv_encoder_util` (unit: `%`)
- `nv_decoder_util` (unit: `%`)
- `nv_remapped_rows_corrected`
- `nv_remapped_rows_uncorrected`
- `nv_remapped_rows_pending`
- `nv_remapped_rows_failure`
- `nv_compute_processes`
- `nv_graphics_processes`
- `nv_violation_power` (unit: `sec`)
- `nv_violation_thermal` (unit: `sec`)
- `nv_violation_sync_boost` (unit: `sec`)
- `nv_violation_board_limit` (unit: `sec`)
- `nv_violation_low_util` (unit: `sec`)
- `nv_violation_reliability` (unit: `sec`)
- `nv_violation_below_app_clock` (unit: `sec`)
- `nv_violation_below_base_clock` (unit: `sec`)
- `nv_nvlink_crc_flit_errors`
- `nv_nvlink_crc_errors`
- `nv_nvlink_ecc_errors`
- `nv_nvlink_replay_errors`
- `nv_nvlink_recovery_errors`
- `nv_nvlink_crc_flit_errors_sum`
- `nv_nvlink_crc_errors_sum`
- `nv_nvlink_ecc_errors_sum`
- `nv_nvlink_replay_errors_sum`
- `nv_nvlink_recovery_errors_sum`

#### Differential Metrics (requires `send_diff_values: true`)
- `nv_ecc_uncorrected_error_diff`
- `nv_ecc_corrected_error_diff`
- `nv_remapped_rows_corrected_diff`
- `nv_remapped_rows_uncorrected_diff`
- `nv_remapped_rows_pending_diff`
- `nv_remapped_rows_failure_diff`
- `nv_violation_power_diff` (unit: `sec`)
- `nv_violation_thermal_diff` (unit: `sec`)
- `nv_violation_sync_boost_diff` (unit: `sec`)
- `nv_violation_board_limit_diff` (unit: `sec`)
- `nv_violation_low_util_diff` (unit: `sec`)
- `nv_violation_reliability_diff` (unit: `sec`)
- `nv_violation_below_app_clock_diff` (unit: `sec`)
- `nv_violation_below_base_clock_diff` (unit: `sec`)
- `nv_nvlink_crc_flit_errors_diff`
- `nv_nvlink_crc_errors_diff`
- `nv_nvlink_ecc_errors_diff`
- `nv_nvlink_replay_errors_diff`
- `nv_nvlink_recovery_errors_diff`
- `nv_nvlink_crc_flit_errors_sum_diff`
- `nv_nvlink_crc_errors_sum_diff`
- `nv_nvlink_ecc_errors_sum_diff`
- `nv_nvlink_replay_errors_sum_diff`
- `nv_nvlink_recovery_errors_sum_diff`
