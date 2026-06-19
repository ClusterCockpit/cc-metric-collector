<!--
---
title: "Nvidia NVML GPM metric collector"
description: Collect metrics for Nvidia GPUs using the NVML GPM interface
categories: [cc-metric-collector]
tags: ['Admin']
weight: 2
hugo_path: docs/reference/cc-metric-collector/collectors/nvidiaGPM.md
---
-->

## `nvidiaGPM` collector

```json
  "nvidia_gpm": {
    "metrics": [
      "nv_fb_mem_used",
      "nv_fan"
    ],
    "exclude_devices": [
      "0","1", "0000000:ff:01.0"
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

The `nvidia_gpm` collector can be configured to leave out specific devices with the `exclude_devices` option. It takes IDs as supplied to the NVML with `nvmlDeviceGetHandleByIndex()` or the PCI address in NVML format (`%08X:%02X:%02X.0`). Commonly only the physical GPUs are monitored. If MIG devices should be analyzed as well, set `process_mig_devices` (adds `stype=mig,stype-id=<mig_index>`). With the options `use_uuid_for_mig_device` and `use_slice_for_mig_device`, the `<mig_index>` can be replaced with the UUID (e.g. `MIG-6a9f7cc8-6d5b-5ce0-92de-750edc4d8849`) or the MIG slice name (e.g. `1g.5gb`).

The metrics sent by the `nvidia_gpm` collector use `accelerator` as `type` tag. For the `type-id`, it uses the device handle index by default. With the `use_pci_info_as_type_id` option, the PCI ID is used instead. If both values should be added as tags, activate the `add_pci_info_tag` option. It uses the device handle index as `type-id` and adds the PCI ID as separate `pci_identifier` tag.

Optionally, it is possible to add the UUID, the board part number and the serial to the meta informations. They are not sent to the sinks (if not configured otherwise).


Available Metrics:
* `nv_gpm_graphics_util`
* `nv_gpm_sm_util`
* `nv_gpm_sm_occupancy`
* `nv_gpm_integer_util`
* `nv_gpm_any_tensor_util`
* `nv_gpm_dfma_tensor_util`
* `nv_gpm_hmma_tensor_util`
* `nv_gpm_imma_tensor_util`
* `nv_gpm_dram_bw_util`
* `nv_gpm_fp64_util`
* `nv_gpm_fp32_util`
* `nv_gpm_fp16_util`