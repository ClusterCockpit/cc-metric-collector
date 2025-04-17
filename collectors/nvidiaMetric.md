<!--
---
title: "Nvidia NVML metric collector"
description: Collect metrics for Nvidia GPUs using the NVML
categories: [cc-metric-collector]
tags: ['Admin']
weight: 2
hugo_path: docs/reference/cc-metric-collector/collectors/nvidia.md
---
-->

## `nvidia` collector

```json
  "nvidia": {
    "exclude_devices": [
      "0","1", "0000000:ff:01.0"
    ],
    "exclude_metrics": [
      "nv_fb_mem_used",
      "nv_fan"
    ],
    "process_mig_devices": false,
    "use_pci_info_as_type_id": true,
    "add_pci_info_tag": false,
    "add_uuid_meta": false,
    "add_board_number_meta": false,
    "add_serial_meta": false,
    "use_uuid_for_mig_device": false,
    "use_slice_for_mig_device": false
  }
```

The `nvidia` collector can be configured to leave out specific devices with the `exclude_devices` option. It takes IDs as supplied to the NVML with `nvmlDeviceGetHandleByIndex()` or the PCI address in NVML format (`%08X:%02X:%02X.0`). Metrics (listed below) that should not be sent to the MetricRouter can be excluded with the `exclude_metrics` option. Commonly only the physical GPUs are monitored. If MIG devices should be analyzed as well, set `process_mig_devices` (adds `stype=mig,stype-id=<mig_index>`). With the options `use_uuid_for_mig_device` and `use_slice_for_mig_device`, the `<mig_index>` can be replaced with the UUID (e.g. `MIG-6a9f7cc8-6d5b-5ce0-92de-750edc4d8849`) or the MIG slice name (e.g. `1g.5gb`).

The metrics sent by the `nvidia` collector use `accelerator` as `type` tag. For the `type-id`, it uses the device handle index by default. With the `use_pci_info_as_type_id` option, the PCI ID is used instead. If both values should be added as tags, activate the `add_pci_info_tag` option. It uses the device handle index as `type-id` and adds the PCI ID as separate `pci_identifier` tag.

Optionally, it is possible to add the UUID, the board part number and the serial to the meta informations. They are not sent to the sinks (if not configured otherwise).


Metrics:
* `nv_util`
* `nv_mem_util`
* `nv_fb_mem_total`
* `nv_fb_mem_used`
* `nv_bar1_mem_total`
* `nv_bar1_mem_used`
* `nv_temp`
* `nv_fan`
* `nv_ecc_mode`
* `nv_perf_state`
* `nv_power_usage`
* `nv_graphics_clock`
* `nv_sm_clock`
* `nv_mem_clock`
* `nv_video_clock`
* `nv_max_graphics_clock`
* `nv_max_sm_clock`
* `nv_max_mem_clock`
* `nv_max_video_clock`
* `nv_ecc_uncorrected_error`
* `nv_ecc_corrected_error`
* `nv_power_max_limit`
* `nv_encoder_util`
* `nv_decoder_util`
* `nv_remapped_rows_corrected`
* `nv_remapped_rows_uncorrected`
* `nv_remapped_rows_pending`
* `nv_remapped_rows_failure`
* `nv_compute_processes`
* `nv_graphics_processes`
* `nv_violation_power`
* `nv_violation_thermal`
* `nv_violation_sync_boost`
* `nv_violation_board_limit`
* `nv_violation_low_util`
* `nv_violation_reliability`
* `nv_violation_below_app_clock`
* `nv_violation_below_base_clock`
* `nv_nvlink_crc_flit_errors`
* `nv_nvlink_crc_errors`
* `nv_nvlink_ecc_errors`
* `nv_nvlink_replay_errors`
* `nv_nvlink_recovery_errors`

Some metrics add the additional sub type tag (`stype`) like the `nv_nvlink_*` metrics set `stype=nvlink,stype-id=<link_number>`. 
